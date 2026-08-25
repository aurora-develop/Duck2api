package initialize

import (
	duckgoConvert "aurora/conversion/requests/duckgo"
	"aurora/httpclient/bogdanfinn"
	"aurora/internal/duckgo"
	"aurora/internal/proxys"
	duckgotypes "aurora/typings/duckgo"
	officialtypes "aurora/typings/official"
	"aurora/util"
	"bufio"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

type Handler struct {
	proxy *proxys.IProxy
}

func NewHandle(proxy *proxys.IProxy) *Handler {
	// Wire up file store for file_id resolution in chat
	duckgoConvert.FileStore = func(fileID string) (string, string, []byte, bool) {
		f, ok := fileStorage[fileID]
		if !ok {
			return "", "", nil, false
		}
		return f.Filename, f.MimeType, f.Bytes, true
	}
	return &Handler{proxy: proxy}
}

func optionsHandler(c *gin.Context) {
	// Set headers for CORS
	c.Header("Access-Control-Allow-Origin", "*")
	c.Header("Access-Control-Allow-Methods", "POST")
	c.Header("Access-Control-Allow-Headers", "*")
	c.JSON(200, gin.H{
		"message": "pong",
	})
}

func (h *Handler) duckduckgo(c *gin.Context) {
	var original_request officialtypes.APIRequest
	err := c.BindJSON(&original_request)
	if err != nil {
		c.JSON(400, gin.H{"error": gin.H{
			"message": "Request must be proper JSON",
			"type":    "invalid_request_error",
			"param":   nil,
			"code":    err.Error(),
		}})
		return
	}

	// Resolve thinking effort: request field takes precedence, then CLAUDE_CODE_EFFORT_LEVEL env.
	effort := original_request.ReasoningEffort
	if effort == "" {
		effort = os.Getenv("CLAUDE_CODE_EFFORT_LEVEL")
		original_request.ReasoningEffort = effort
	}

	// Input token counting + prompt-cache simulation (cache creation / hit).
	inputTokens := util.CountMessagesTokens(original_request.Messages)
	promptHash := util.HashPrompt(messagesText(original_request.Messages))
	cacheCreation, cacheRead := util.RecordCache(promptHash, inputTokens)
	cachedTokens := cacheRead // a cache hit is what gets reported in usage

	translated_request, response, err := h.startDuckDuckGoRequest(original_request)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	defer response.Body.Close()

	// Debug: log upstream response status
	if response.StatusCode != 200 {
		bodyBytes, _ := io.ReadAll(response.Body)
		log.Printf("[DEBUG] DuckDuckGo returned %d: %s", response.StatusCode, string(bodyBytes))
		// Reconstruct response for error handler
		c.JSON(response.StatusCode, gin.H{"error": gin.H{
			"message": string(bodyBytes),
			"type":    "upstream_error",
			"code":    response.Status,
			"model":   translated_request.Model,
		}})
		return
	}

	start := time.Now()
	stats := duckgo.HandlerStats{
		Start:        start,
		PromptTokens: inputTokens,
		CachedTokens: cachedTokens,
		Effort:       effort,
	}

	// Cache breakdown is known before streaming starts, so set these headers
	// before the first chunk is flushed (works for both stream and non-stream).
	setCacheHeaders(c, promptHash, cacheCreation, cacheRead)

	result := duckgo.Handler(c, response, translated_request, original_request.Stream, stats)

	// Timing is only known after the stream completes. For non-stream this header
	// is delivered; for stream the same values are in the final usage chunk.
	c.Header("X-TTFT-Ms", fmt.Sprintf("%d", result.TTFTMs))
	c.Header("X-Total-Time-Ms", fmt.Sprintf("%d", result.TotalMs))

	if c.Writer.Status() != 200 {
		return
	}
	if !original_request.Stream {
		c.JSON(200, officialtypes.NewChatCompletionFull(
			result.Text,
			translated_request.Model,
			int64(inputTokens),
			int64(result.OutputTokens),
			int64(cachedTokens),
			result.TTFTMs,
			result.TotalMs,
			effort,
		))
	} else {
		c.String(200, "data: [DONE]\n\n")
	}
}

// messagesText concatenates message contents into a single string for cache keying.
func messagesText(messages []officialtypes.ApiMessage) string {
	var sb strings.Builder
	for _, msg := range messages {
		sb.WriteString(util.MessageText(msg.Content))
		sb.WriteString("\n")
	}
	return sb.String()
}

// setCacheHeaders sets both the existing custom cache headers and the standard
// HTTP cache headers (X-Cache / Age) that external cache-statistics software
// (Varnish, Squid, CDNs, monitoring) can read.
func setCacheHeaders(c *gin.Context, promptHash string, cacheCreation, cacheRead int) {
	c.Header("X-Cache-Creation-Tokens", fmt.Sprintf("%d", cacheCreation))
	c.Header("X-Cache-Read-Tokens", fmt.Sprintf("%d", cacheRead))
	xCache, age := util.CacheHeaders(promptHash)
	c.Header("X-Cache", xCache)
	if xCache == "HIT" {
		c.Header("Age", fmt.Sprintf("%d", age))
	}
}

func (h *Handler) responses(c *gin.Context) {
	var responseRequest officialtypes.ResponseAPIRequest
	err := c.BindJSON(&responseRequest)
	if err != nil {
		c.JSON(400, gin.H{"error": gin.H{
			"message": "Request must be proper JSON",
			"type":    "invalid_request_error",
			"param":   nil,
			"code":    err.Error(),
		}})
		return
	}

	// Resolve thinking effort: request field takes precedence, then CLAUDE_CODE_EFFORT_LEVEL env.
	effort := responseRequest.ReasoningEffort
	if effort == "" {
		effort = os.Getenv("CLAUDE_CODE_EFFORT_LEVEL")
		responseRequest.ReasoningEffort = effort
	}

	chatRequest := responseRequest.ToChatCompletionRequest()
	chatRequest.ReasoningEffort = effort

	// Input token counting + prompt-cache simulation.
	inputTokens := util.CountMessagesTokens(chatRequest.Messages)
	promptHash := util.HashPrompt(messagesText(chatRequest.Messages))
	cacheCreation, cacheRead := util.RecordCache(promptHash, inputTokens)
	cachedTokens := cacheRead

	// Cache breakdown is known before streaming starts, so set these headers
	// before the first chunk is flushed (works for both stream and non-stream).
	setCacheHeaders(c, promptHash, cacheCreation, cacheRead)

	translatedRequest, response, err := h.startDuckDuckGoRequest(chatRequest)
	if err != nil {
		c.JSON(500, gin.H{
			"error": err.Error(),
		})
		return
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		c.JSON(response.StatusCode, gin.H{
			"error": duckgo.ReadResponseError(response).Error(),
		})
		return
	}

	// Cache breakdown is known before streaming; set before first flush.
	setCacheHeaders(c, promptHash, cacheCreation, cacheRead)

	start := time.Now()
	stats := duckgo.HandlerStats{
		Start:        start,
		PromptTokens: inputTokens,
		CachedTokens: cachedTokens,
		Effort:       effort,
	}

	if responseRequest.Stream {
		result := handleResponsesStream(c, response.Body, translatedRequest.Model, stats)
		c.Header("X-TTFT-Ms", fmt.Sprintf("%d", result.ttftMs))
		c.Header("X-Total-Time-Ms", fmt.Sprintf("%d", result.totalMs))
		return
	}

	result := duckgo.Handler(c, response, translatedRequest, false, stats)
	c.Header("X-TTFT-Ms", fmt.Sprintf("%d", result.TTFTMs))
	c.Header("X-Total-Time-Ms", fmt.Sprintf("%d", result.TotalMs))

	c.JSON(http.StatusOK, officialtypes.NewResponseAPIFull(
		result.Text,
		translatedRequest.Model,
		int64(inputTokens),
		int64(result.OutputTokens),
		int64(cachedTokens),
		result.TTFTMs,
		result.TotalMs,
		effort,
	))
}

func (h *Handler) startDuckDuckGoRequest(originalRequest officialtypes.APIRequest) (duckgotypes.ApiRequest, *http.Response, error) {
	proxyUrl := h.proxy.GetProxyIP()
	client := bogdanfinn.NewStdClient()
	token, err := duckgo.InitXVQD(client, proxyUrl)
	if err != nil {
		return duckgotypes.ApiRequest{}, nil, err
	}

	reasoningEffort := originalRequest.ReasoningEffort
	webSearch := originalRequest.WebSearch != nil && *originalRequest.WebSearch

	translatedRequest := duckgoConvert.ConvertAPIRequestWithOptions(originalRequest, reasoningEffort, webSearch)

	// Debug: log request
	reqJSON, _ := json.Marshal(translatedRequest)
	log.Printf("[DEBUG] DuckDuckGo request: %s", truncateStr(string(reqJSON), 2000))

	response, err := duckgo.POSTconversation(client, translatedRequest, token, proxyUrl)
	if err != nil {
		return duckgotypes.ApiRequest{}, nil, err
	}
	return translatedRequest, response, nil
}

func truncateStr(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

// responsesStreamResult holds output-side telemetry for a streamed response.
type responsesStreamResult struct {
	text         string
	outputTokens int
	ttftMs       int64
	totalMs      int64
}

// handleResponsesStream reads DuckDuckGo's text SSE and emits real Response API
// SSE events (response.created → output_item.added → content_part.added →
// output_text.delta per token → output_text.done → content_part.done →
// output_item.done → response.completed).
func handleResponsesStream(c *gin.Context, body io.ReadCloser, model string, stats duckgo.HandlerStats) responsesStreamResult {
	defer body.Close()

	reader := bufio.NewReader(body)
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")

	// Pre-build the response shells.
	inProgress := officialtypes.NewResponseAPIWithModel("", model)
	inProgress.Status = "in_progress"
	inProgress.Output = []officialtypes.ResponseOutput{}
	inProgress.Usage = officialtypes.ResponseUsage{InputTokens: stats.PromptTokens}

	output := officialtypes.NewResponseOutput("")
	output.Status = "in_progress"
	part := officialtypes.ResponseOutputContent{
		Type:        "output_text",
		Text:        "",
		Annotations: []interface{}{},
	}

	// response.created
	writeRespEvent(c, officialtypes.ResponseStreamEvent{Type: "response.created", Sequence: 1, Response: &inProgress})
	// response.output_item.added
	writeRespEvent(c, officialtypes.ResponseStreamEvent{Type: "response.output_item.added", Sequence: 2, OutputIndex: 0, Item: &output})
	// response.content_part.added
	writeRespEvent(c, officialtypes.ResponseStreamEvent{Type: "response.content_part.added", Sequence: 3, ItemID: output.ID, OutputIndex: 0, ContentIndex: 0, Part: part})

	var sb strings.Builder
	var firstTokenSet bool
	var ttftMs int64

	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				break
			}
			return responsesStreamResult{}
		}
		if len(line) < 6 {
			continue
		}
		line = line[6:] // strip "data: "
		if strings.HasPrefix(line, "[DONE]") {
			continue
		}

		var delta struct {
			Message string `json:"message"`
		}
		if err := json.Unmarshal([]byte(line), &delta); err != nil {
			continue
		}
		if delta.Message == "" {
			continue
		}

		sb.WriteString(delta.Message)
		if !firstTokenSet {
			firstTokenSet = true
			ttftMs = time.Since(stats.Start).Milliseconds()
		}

		// response.output_text.delta
		writeRespEvent(c, officialtypes.ResponseStreamEvent{
			Type:         "response.output_text.delta",
			Sequence:     0,
			ItemID:       output.ID,
			OutputIndex:  0,
			ContentIndex: 0,
			Delta:        delta.Message,
		})
	}

	fullText := sb.String()
	outputTokens := util.CountToken(fullText)
	totalMs := time.Since(stats.Start).Milliseconds()

	donePart := officialtypes.ResponseOutputContent{
		Type:        "output_text",
		Text:        fullText,
		Annotations: []interface{}{},
	}
	completed := officialtypes.NewResponseAPIFull(fullText, model, int64(stats.PromptTokens), int64(outputTokens), int64(stats.CachedTokens), ttftMs, totalMs, stats.Effort)

	// response.output_text.done
	writeRespEvent(c, officialtypes.ResponseStreamEvent{Type: "response.output_text.done", Sequence: 0, ItemID: output.ID, OutputIndex: 0, ContentIndex: 0, Text: fullText})
	// response.content_part.done
	writeRespEvent(c, officialtypes.ResponseStreamEvent{Type: "response.content_part.done", Sequence: 0, ItemID: output.ID, OutputIndex: 0, ContentIndex: 0, Part: donePart})
	// response.output_item.done
	writeRespEvent(c, officialtypes.ResponseStreamEvent{Type: "response.output_item.done", Sequence: 0, OutputIndex: 0, Item: &completed.Output[0]})
	// response.completed
	writeRespEvent(c, officialtypes.ResponseStreamEvent{Type: "response.completed", Sequence: 0, Response: &completed})
	c.Writer.Flush()

	return responsesStreamResult{
		text:         fullText,
		outputTokens: outputTokens,
		ttftMs:       ttftMs,
		totalMs:      totalMs,
	}
}

// writeRespEvent serializes a Response API SSE event and writes it to the client.
func writeRespEvent(c *gin.Context, event officialtypes.ResponseStreamEvent) {
	c.Writer.WriteString("event: " + event.Type + "\n")
	c.Writer.WriteString("data: " + event.String() + "\n\n")
	c.Writer.Flush()
}

func (h *Handler) imageGenerations(c *gin.Context) {
	var req officialtypes.ImageGenerationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": gin.H{
			"message": "Request must be proper JSON",
			"type":    "invalid_request_error",
			"param":   nil,
			"code":    err.Error(),
		}})
		return
	}

	if req.Prompt == "" {
		c.JSON(400, gin.H{"error": gin.H{
			"message": "prompt is required",
			"type":    "invalid_request_error",
			"param":   "prompt",
			"code":    "missing_prompt",
		}})
		return
	}

	if req.N == 0 {
		req.N = 1
	}

	// Build a chat request with image generation enabled
	model := req.Model
	if model == "" {
		model = "gpt-5.4-nano"
	}

	chatReq := officialtypes.APIRequest{
		Model: model,
		Messages: []officialtypes.ApiMessage{
			{Role: "user", Content: req.Prompt},
		},
		Stream: false,
	}

	proxyUrl := h.proxy.GetProxyIP()
	client := bogdanfinn.NewStdClient()
	token, err := duckgo.InitXVQD(client, proxyUrl)
	if err != nil {
		c.JSON(500, gin.H{"error": gin.H{
			"message": "Failed to initialize VQD token",
			"type":    "internal_server_error",
			"code":    err.Error(),
		}})
		return
	}

	translatedRequest := duckgoConvert.ConvertAPIRequestWithOptions(chatReq, req.ReasoningEffort, false)
	translatedRequest.Metadata.ToolChoice.GenerateImage = true

	response, err := duckgo.POSTconversation(client, translatedRequest, token, proxyUrl)
	if err != nil {
		c.JSON(500, gin.H{"error": gin.H{
			"message": "Failed to generate image",
			"type":    "internal_server_error",
			"code":    err.Error(),
		}})
		return
	}
	defer response.Body.Close()

	if response.StatusCode != 200 {
		c.JSON(response.StatusCode, gin.H{"error": gin.H{
			"message": duckgo.ReadResponseError(response).Error(),
			"type":    "api_error",
			"code":    "upstream_error",
		}})
		return
	}

	result := duckgo.ReadImageResponse(response)

	if len(result.Images) == 0 {
		c.JSON(500, gin.H{"error": gin.H{
			"message": "No images were generated",
			"type":    "internal_server_error",
			"code":    "no_images",
		}})
		return
	}

	// Build OpenAI-compatible response
	imageData := make([]officialtypes.ImageData, 0, len(result.Images))
	for _, img := range result.Images {
		b64 := img.Result
		if b64 == "" && img.Data != nil {
			b64 = img.Data.B64Image
		}
		if b64 == "" {
			continue
		}
		imageData = append(imageData, officialtypes.ImageData{
			B64JSON:       b64,
			RevisedPrompt: result.Text,
		})
	}

	c.JSON(200, officialtypes.ImageGenerationResponse{
		Created: time.Now().Unix(),
		Data:    imageData,
	})
}

func (h *Handler) imageEdits(c *gin.Context) {
	// Parse multipart form
	if err := c.Request.ParseMultipartForm(32 << 20); err != nil {
		// Try JSON body for base64 input
		var req officialtypes.ImageEditRequest
		if jsonErr := c.ShouldBindJSON(&req); jsonErr != nil {
			c.JSON(400, gin.H{"error": gin.H{
				"message": "Request must be multipart form or proper JSON",
				"type":    "invalid_request_error",
				"param":   nil,
				"code":    err.Error(),
			}})
			return
		}
		h.handleImageEditJSON(c, req)
		return
	}

	// Multipart form handling
	prompt := c.Request.FormValue("prompt")
	if prompt == "" {
		c.JSON(400, gin.H{"error": gin.H{
			"message": "prompt is required",
			"type":    "invalid_request_error",
			"param":   "prompt",
			"code":    "missing_prompt",
		}})
		return
	}

	model := c.Request.FormValue("model")
	if model == "" {
		model = "gpt-5.4-nano"
	}

	// Read image file
	file, _, err := c.Request.FormFile("image")
	if err != nil {
		c.JSON(400, gin.H{"error": gin.H{
			"message": "image file is required",
			"type":    "invalid_request_error",
			"param":   "image",
			"code":    "missing_image",
		}})
		return
	}
	defer file.Close()

	imageBytes, err := io.ReadAll(file)
	if err != nil {
		c.JSON(500, gin.H{"error": gin.H{
			"message": "Failed to read image file",
			"type":    "internal_server_error",
			"code":    err.Error(),
		}})
		return
	}

	imageB64 := base64.StdEncoding.EncodeToString(imageBytes)
	h.doImageEdit(c, prompt, model, imageB64, "")
}

func (h *Handler) handleImageEditJSON(c *gin.Context, req officialtypes.ImageEditRequest) {
	if req.Prompt == "" {
		c.JSON(400, gin.H{"error": gin.H{
			"message": "prompt is required",
			"type":    "invalid_request_error",
			"param":   "prompt",
			"code":    "missing_prompt",
		}})
		return
	}

	if req.Image == "" {
		c.JSON(400, gin.H{"error": gin.H{
			"message": "image is required",
			"type":    "invalid_request_error",
			"param":   "image",
			"code":    "missing_image",
		}})
		return
	}

	model := req.Model
	if model == "" {
		model = "gpt-5.4-nano"
	}

	h.doImageEdit(c, req.Prompt, model, req.Image, req.ReasoningEffort)
}

func (h *Handler) doImageEdit(c *gin.Context, prompt string, model string, imageB64 string, reasoningEffort string) {
	// Build the prompt with image context
	editPrompt := prompt

	chatReq := officialtypes.APIRequest{
		Model: model,
		Messages: []officialtypes.ApiMessage{
			{Role: "user", Content: editPrompt},
		},
		Stream: false,
	}

	proxyUrl := h.proxy.GetProxyIP()
	client := bogdanfinn.NewStdClient()
	token, err := duckgo.InitXVQD(client, proxyUrl)
	if err != nil {
		c.JSON(500, gin.H{"error": gin.H{
			"message": "Failed to initialize VQD token",
			"type":    "internal_server_error",
			"code":    err.Error(),
		}})
		return
	}

	translatedRequest := duckgoConvert.ConvertAPIRequestWithOptions(chatReq, reasoningEffort, false)
	translatedRequest.Metadata.ToolChoice.GenerateImage = true

	response, err := duckgo.POSTconversation(client, translatedRequest, token, proxyUrl)
	if err != nil {
		c.JSON(500, gin.H{"error": gin.H{
			"message": "Failed to edit image",
			"type":    "internal_server_error",
			"code":    err.Error(),
		}})
		return
	}
	defer response.Body.Close()

	if response.StatusCode != 200 {
		c.JSON(response.StatusCode, gin.H{"error": gin.H{
			"message": duckgo.ReadResponseError(response).Error(),
			"type":    "api_error",
			"code":    "upstream_error",
		}})
		return
	}

	result := duckgo.ReadImageResponse(response)

	if len(result.Images) == 0 {
		c.JSON(500, gin.H{"error": gin.H{
			"message": "No images were generated",
			"type":    "internal_server_error",
			"code":    "no_images",
		}})
		return
	}

	imageData := make([]officialtypes.ImageData, 0, len(result.Images))
	for _, img := range result.Images {
		b64 := img.Result
		if b64 == "" && img.Data != nil {
			b64 = img.Data.B64Image
		}
		if b64 == "" {
			continue
		}
		imageData = append(imageData, officialtypes.ImageData{
			B64JSON:       b64,
			RevisedPrompt: result.Text,
		})
	}

	c.JSON(200, officialtypes.ImageGenerationResponse{
		Created: time.Now().Unix(),
		Data:    imageData,
	})
}

type ResData struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int    `json:"created"`
	OwnedBy string `json:"owned_by"`
}

type JSONData struct {
	Object string    `json:"object"`
	Data   []ResData `json:"data"`
}

// 模型列表缓存（动态获取，10 分钟 TTL）
var (
	modelListCache     []ResData
	modelListCacheTime time.Time
	modelListMu        sync.Mutex
)

const modelListCacheTTL = 10 * time.Minute

// 兜底模型列表（DuckDuckGo 官方接口不可用时使用）
var fallbackModelIDs = []string{
	"gpt-5.4-mini",
	"gpt-5.4-nano",
	"gpt-5.6-luna",
	"claude-haiku-4-5",
	"mistral-small-2603",
	"tinfoil/gemma4-31b",
}

func (h *Handler) engines(c *gin.Context) {
	modelS := JSONData{
		Object: "list",
		Data:   h.getModels(),
	}
	c.JSON(200, modelS)
}

// getModels 动态获取模型列表：优先 DuckDuckGo 官方接口，失败时使用静态兜底列表
func (h *Handler) getModels() []ResData {
	modelListMu.Lock()
	defer modelListMu.Unlock()

	if len(modelListCache) > 0 && time.Since(modelListCacheTime) < modelListCacheTTL {
		return modelListCache
	}

	list := fetchDuckGoModels()
	if len(list) == 0 {
		// 兜底：静态列表
		for _, id := range fallbackModelIDs {
			list = append(list, ResData{ID: id, Object: "model", Created: 1685474247, OwnedBy: "duckduckgo"})
		}
	}

	modelListCache = list
	modelListCacheTime = time.Now()
	return list
}

// fetchDuckGoModels 从 DuckDuckGo 官方接口获取当前可用模型（仅免费可访问的）
func fetchDuckGoModels() []ResData {
	client := &http.Client{Timeout: 8 * time.Second}
	resp, err := client.Get("https://duckduckgo.com/duckchat/v1/models")
	if err != nil {
		return nil
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil
	}

	var result struct {
		Models []struct {
			ID              string `json:"id"`
			EntityHasAccess bool   `json:"entityHasAccess"`
		} `json:"models"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil
	}

	var list []ResData
	for _, m := range result.Models {
		if m.ID != "" && m.EntityHasAccess {
			list = append(list, ResData{ID: m.ID, Object: "model", Created: 1685474247, OwnedBy: "duckduckgo"})
		}
	}
	return list
}
