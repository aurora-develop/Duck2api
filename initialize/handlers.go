package initialize

import (
	duckgoConvert "aurora/conversion/requests/duckgo"
	"aurora/httpclient/bogdanfinn"
	"aurora/internal/duckgo"
	"aurora/internal/proxys"
	duckgotypes "aurora/typings/duckgo"
	officialtypes "aurora/typings/official"
	"net/http"

	"github.com/gin-gonic/gin"
)

type Handler struct {
	proxy *proxys.IProxy
}

func NewHandle(proxy *proxys.IProxy) *Handler {
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
	translated_request, response, err := h.startDuckDuckGoRequest(original_request)
	if err != nil {
		c.JSON(500, gin.H{
			"error": err.Error(),
		})
		return
	}
	defer response.Body.Close()

	if duckgo.Handle_request_error(c, response) {
		return
	}
	response_part := duckgo.Handler(c, response, translated_request, original_request.Stream)
	if c.Writer.Status() != 200 {
		return
	}
	if !original_request.Stream {
		c.JSON(200, officialtypes.NewChatCompletionWithModel(response_part, translated_request.Model))
	} else {
		c.String(200, "data: [DONE]\n\n")
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

	chatRequest := responseRequest.ToChatCompletionRequest()
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

	responseText := duckgo.ReadResponseText(response)

	if responseRequest.Stream {
		writeResponsesStream(c, responseText, translatedRequest.Model)
		return
	}

	c.JSON(http.StatusOK, officialtypes.NewResponseAPIWithModel(responseText, translatedRequest.Model))
}

func (h *Handler) startDuckDuckGoRequest(originalRequest officialtypes.APIRequest) (duckgotypes.ApiRequest, *http.Response, error) {
	proxyUrl := h.proxy.GetProxyIP()
	client := bogdanfinn.NewStdClient()
	token, err := duckgo.InitXVQD(client, proxyUrl)
	if err != nil {
		return duckgotypes.ApiRequest{}, nil, err
	}

	translatedRequest := duckgoConvert.ConvertAPIRequest(originalRequest)
	response, err := duckgo.POSTconversation(client, translatedRequest, token, proxyUrl)
	if err != nil {
		return duckgotypes.ApiRequest{}, nil, err
	}
	return translatedRequest, response, nil
}

func writeResponsesStream(c *gin.Context, text string, model string) {
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")

	response := officialtypes.NewResponseAPIWithModel("", model)
	response.Status = "in_progress"
	response.Output = []officialtypes.ResponseOutput{}
	output := officialtypes.NewResponseOutput("")
	output.Status = "in_progress"
	part := officialtypes.ResponseOutputContent{
		Type:        "output_text",
		Text:        "",
		Annotations: []interface{}{},
	}
	donePart := officialtypes.ResponseOutputContent{
		Type:        "output_text",
		Text:        text,
		Annotations: []interface{}{},
	}
	events := []officialtypes.ResponseStreamEvent{
		{Type: "response.created", Sequence: 1, Response: &response},
		{Type: "response.output_item.added", Sequence: 2, OutputIndex: 0, Item: &output},
		{Type: "response.content_part.added", Sequence: 3, ItemID: output.ID, OutputIndex: 0, ContentIndex: 0, Part: part},
		{Type: "response.output_text.delta", Sequence: 4, ItemID: output.ID, OutputIndex: 0, ContentIndex: 0, Delta: text},
		{Type: "response.output_text.done", Sequence: 5, ItemID: output.ID, OutputIndex: 0, ContentIndex: 0, Text: text},
		{Type: "response.content_part.done", Sequence: 6, ItemID: output.ID, OutputIndex: 0, ContentIndex: 0, Part: donePart},
	}

	completed := officialtypes.NewResponseAPIWithModel(text, model)
	events = append(events,
		officialtypes.ResponseStreamEvent{Type: "response.output_item.done", Sequence: 7, OutputIndex: 0, Item: &completed.Output[0]},
		officialtypes.ResponseStreamEvent{Type: "response.completed", Sequence: 8, Response: &completed},
	)

	for _, event := range events {
		c.Writer.WriteString("event: " + event.Type + "\n")
		c.Writer.WriteString("data: " + event.String() + "\n\n")
		c.Writer.Flush()
	}
}

func (h *Handler) engines(c *gin.Context) {
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

	modelS := JSONData{
		Object: "list",
	}
	var resModelList []ResData

	// Supported models
	modelIDs := []string{
		"gpt-4o-mini",
		"gpt-5-mini",
			"gpt-5.4-mini",
		"tinfoil/gpt-oss-120b",
		"gpt-3.5-turbo-0125",
		"claude-3-haiku-20240307",
		"claude-haiku-4-5",
		"llama-3.3-70b",
		"llama-4-scout",
		"mistral-small",
		"meta-llama/Llama-4-Scout-17B-16E-Instruct",
		"mistralai/Mistral-Small-24B-Instruct-2501",
	}

	for _, modelID := range modelIDs {
		resModelList = append(resModelList, ResData{
			ID:      modelID,
			Object:  "model",
			Created: 1685474247,
			OwnedBy: "duckduckgo",
		})
	}

	modelS.Data = resModelList
	c.JSON(200, modelS)
}
