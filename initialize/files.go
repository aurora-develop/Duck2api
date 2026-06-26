package initialize

import (
	"aurora/httpclient"
	"aurora/httpclient/bogdanfinn"
	"aurora/internal/duckgo"
	officialtypes "aurora/typings/official"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// FileObject represents an uploaded file in the /v1/files response
type FileObject struct {
	ID        string `json:"id"`
	Object    string `json:"object"`
	Bytes     int    `json:"bytes"`
	CreatedAt int64  `json:"created_at"`
	Filename  string `json:"filename"`
	Purpose   string `json:"purpose"`
}

// TranscriptionResponse represents the /v1/audio/transcriptions response
type TranscriptionResponse struct {
	Text string `json:"text"`
}

// StoredFile holds a file uploaded via /v1/files
type StoredFile struct {
	ID       string
	Filename string
	Bytes    []byte
	MimeType string
	Created  int64
}

// fileStorage is a simple in-memory file store
var fileStorage = make(map[string]*StoredFile)

func (h *Handler) filesUpload(c *gin.Context) {
	// Parse multipart form
	if err := c.Request.ParseMultipartForm(32 << 20); err != nil {
		c.JSON(400, gin.H{"error": gin.H{
			"message": "Request must be multipart/form-data",
			"type":    "invalid_request_error",
			"code":    err.Error(),
		}})
		return
	}

	file, header, err := c.Request.FormFile("file")
	if err != nil {
		c.JSON(400, gin.H{"error": gin.H{
			"message": "file is required",
			"type":    "invalid_request_error",
			"param":   "file",
			"code":    "missing_file",
		}})
		return
	}
	defer file.Close()

	purpose := c.Request.FormValue("purpose")
	if purpose == "" {
		purpose = "assistants"
	}

	fileBytes, err := io.ReadAll(file)
	if err != nil {
		c.JSON(500, gin.H{"error": gin.H{
			"message": "Failed to read file",
			"type":    "internal_server_error",
			"code":    err.Error(),
		}})
		return
	}

	// Generate file ID
	fileID := fmt.Sprintf("file-%d", time.Now().UnixNano())

	// Detect mime type
	mimeType := header.Header.Get("Content-Type")
	if mimeType == "" || mimeType == "application/octet-stream" {
		mimeType = detectMimeType(header.Filename)
	}

	// Store file
	fileStorage[fileID] = &StoredFile{
		ID:       fileID,
		Filename: header.Filename,
		Bytes:    fileBytes,
		MimeType: mimeType,
		Created:  time.Now().Unix(),
	}

	c.JSON(200, FileObject{
		ID:        fileID,
		Object:    "file",
		Bytes:     len(fileBytes),
		CreatedAt: time.Now().Unix(),
		Filename:  header.Filename,
		Purpose:   purpose,
	})
}

func (h *Handler) filesList(c *gin.Context) {
	files := make([]FileObject, 0, len(fileStorage))
	for _, f := range fileStorage {
		files = append(files, FileObject{
			ID:        f.ID,
			Object:    "file",
			Bytes:     len(f.Bytes),
			CreatedAt: f.Created,
			Filename:  f.Filename,
			Purpose:   "assistants",
		})
	}
	c.JSON(200, gin.H{
		"object": "list",
		"data":   files,
	})
}

func (h *Handler) filesGet(c *gin.Context) {
	fileID := c.Param("file_id")
	f, ok := fileStorage[fileID]
	if !ok {
		c.JSON(404, gin.H{"error": gin.H{
			"message": "File not found",
			"type":    "not_found_error",
			"code":    "file_not_found",
		}})
		return
	}
	c.JSON(200, FileObject{
		ID:        f.ID,
		Object:    "file",
		Bytes:     len(f.Bytes),
		CreatedAt: f.Created,
		Filename:  f.Filename,
		Purpose:   "assistants",
	})
}

func (h *Handler) filesDelete(c *gin.Context) {
	fileID := c.Param("file_id")
	if _, ok := fileStorage[fileID]; !ok {
		c.JSON(404, gin.H{"error": gin.H{
			"message": "File not found",
			"type":    "not_found_error",
			"code":    "file_not_found",
		}})
		return
	}
	delete(fileStorage, fileID)
	c.JSON(200, gin.H{
		"id":      fileID,
		"object":  "file",
		"deleted": true,
	})
}

func (h *Handler) filesContent(c *gin.Context) {
	fileID := c.Param("file_id")
	f, ok := fileStorage[fileID]
	if !ok {
		c.JSON(404, gin.H{"error": gin.H{
			"message": "File not found",
			"type":    "not_found_error",
			"code":    "file_not_found",
		}})
		return
	}
	c.Header("Content-Type", f.MimeType)
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%q", f.Filename))
	c.Data(200, f.MimeType, f.Bytes)
}

// audioTranscriptions handles POST /v1/audio/transcriptions
func (h *Handler) audioTranscriptions(c *gin.Context) {
	// Parse multipart form
	if err := c.Request.ParseMultipartForm(32 << 20); err != nil {
		c.JSON(400, gin.H{"error": gin.H{
			"message": "Request must be multipart/form-data",
			"type":    "invalid_request_error",
			"code":    err.Error(),
		}})
		return
	}

	file, header, err := c.Request.FormFile("file")
	if err != nil {
		c.JSON(400, gin.H{"error": gin.H{
			"message": "file is required",
			"type":    "invalid_request_error",
			"param":   "file",
			"code":    "missing_file",
		}})
		return
	}
	defer file.Close()

	audioBytes, err := io.ReadAll(file)
	if err != nil {
		c.JSON(500, gin.H{"error": gin.H{
			"message": "Failed to read audio file",
			"type":    "internal_server_error",
			"code":    err.Error(),
		}})
		return
	}

	// Detect audio content type
	contentType := header.Header.Get("Content-Type")
	if contentType == "" || contentType == "application/octet-stream" {
		contentType = detectAudioMimeType(header.Filename)
	}

	// Call Duck.ai dictation endpoint
	text, err := h.callDictation(audioBytes, contentType)
	if err != nil {
		c.JSON(500, gin.H{"error": gin.H{
			"message": "Transcription failed",
			"type":    "internal_server_error",
			"code":    err.Error(),
		}})
		return
	}

	c.JSON(200, TranscriptionResponse{
		Text: text,
	})
}

func (h *Handler) callDictation(audioBytes []byte, contentType string) (string, error) {
	proxyUrl := h.proxy.GetProxyIP()
	client := bogdanfinn.NewStdClient()
	if proxyUrl != "" {
		client.SetProxy(proxyUrl)
	}

	maxRetries := 3
	for i := 0; i <= maxRetries; i++ {
		token, err := duckgo.InitXVQD(client, proxyUrl)
		if err != nil {
			return "", fmt.Errorf("failed to init VQD: %w", err)
		}

		header := make(httpclient.AuroraHeaders)
		header.Set("Content-Type", contentType)
		header.Set("accept", "application/json")
		header.Set("origin", "https://duck.ai")
		header.Set("referer", "https://duck.ai/")
		header.Set("user-agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/149.0.0.0 Safari/537.36")
		header.Set("x-vqd-hash-1", token)
		header.Set("x-ddg-journey-id", duckgo.RandomHex(16))
		header.Set("x-fe-signals", duckgo.CreateFESignals())

		if feVersion, err := duckgo.InitFEVersion(client, ""); err == nil && feVersion != "" {
			header.Set("x-fe-version", feVersion)
		}

		log.Printf("[DEBUG] Dictation attempt %d: %d bytes, contentType=%s", i+1, len(audioBytes), contentType)
		resp, err := client.Request(httpclient.POST, "https://duck.ai/duckchat/v1/dictation", header, nil, bytes.NewReader(audioBytes))
		if err != nil {
			return "", fmt.Errorf("dictation request failed: %w", err)
		}

		log.Printf("[DEBUG] Dictation response: %d", resp.StatusCode)
		if resp.StatusCode == 418 || resp.StatusCode == 429 {
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			log.Printf("[DEBUG] Dictation retry body: %s", string(body))
			duckgo.ResetXVQD()
			continue
		}

		if resp.StatusCode != 200 {
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			return "", fmt.Errorf("dictation returned %d: %s", resp.StatusCode, string(body))
		}

		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			return "", fmt.Errorf("failed to read dictation response: %w", err)
		}

		// Try to parse as JSON
		var result map[string]interface{}
		if err := json.Unmarshal(body, &result); err != nil {
			text := strings.TrimSpace(string(body))
			if text != "" {
				return text, nil
			}
			return "", fmt.Errorf("empty dictation response")
		}

		for _, key := range []string{"text", "transcription", "result", "content"} {
			if val, ok := result[key]; ok {
				if s, ok := val.(string); ok && s != "" {
					return s, nil
				}
			}
		}

		return string(body), nil
	}

	return "", fmt.Errorf("dictation failed after %d retries", maxRetries)
}

// chatWithFiles handles chat completions with file references
func (h *Handler) chatWithFiles(c *gin.Context) {
	var req struct {
		officialtypes.APIRequest
		FileIDs []string `json:"file_ids,omitempty"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": gin.H{
			"message": "Request must be proper JSON",
			"type":    "invalid_request_error",
			"code":    err.Error(),
		}})
		return
	}

	// Load files and append to first user message
	if len(req.FileIDs) > 0 {
		for _, fileID := range req.FileIDs {
			f, ok := fileStorage[fileID]
			if !ok {
				c.JSON(400, gin.H{"error": gin.H{
					"message": fmt.Sprintf("File %s not found", fileID),
					"type":    "invalid_request_error",
					"code":    "file_not_found",
				}})
				return
			}
			// Append file content as context to the last user message
			if len(req.Messages) > 0 {
				lastIdx := len(req.Messages) - 1
				fileContext := fmt.Sprintf("\n\n[Attached file: %s]\n%s", f.Filename, string(f.Bytes))
				if strContent, ok := req.Messages[lastIdx].Content.(string); ok {
					req.Messages[lastIdx].Content = strContent + fileContext
				}
			}
		}
	}

	// Process as normal chat
	translated_request, response, err := h.startDuckDuckGoRequest(req.APIRequest)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	defer response.Body.Close()

	if duckgo.Handle_request_error(c, response) {
		return
	}
	response_part := duckgo.Handler(c, response, translated_request, req.Stream)
	if c.Writer.Status() != 200 {
		return
	}
	if !req.Stream {
		c.JSON(200, officialtypes.NewChatCompletionWithModel(response_part, translated_request.Model))
	} else {
		c.String(200, "data: [DONE]\n\n")
	}
}

func detectMimeType(filename string) string {
	ext := strings.ToLower(filepath.Ext(filename))
	switch ext {
	case ".pdf":
		return "application/pdf"
	case ".txt":
		return "text/plain"
	case ".json":
		return "application/json"
	case ".csv":
		return "text/csv"
	case ".md":
		return "text/markdown"
	case ".html", ".htm":
		return "text/html"
	case ".xml":
		return "text/xml"
	case ".py":
		return "text/x-python"
	case ".js":
		return "application/javascript"
	case ".ts":
		return "application/typescript"
	case ".go":
		return "text/x-go"
	case ".java":
		return "text/x-java"
	case ".c":
		return "text/x-c"
	case ".cpp":
		return "text/x-c++"
	case ".rs":
		return "text/x-rust"
	case ".rb":
		return "text/x-ruby"
	case ".yaml", ".yml":
		return "text/yaml"
	case ".toml":
		return "text/x-toml"
	case ".sql":
		return "application/sql"
	case ".sh":
		return "application/x-sh"
	case ".doc", ".docx":
		return "application/msword"
	case ".xls", ".xlsx":
		return "application/vnd.ms-excel"
	case ".ppt", ".pptx":
		return "application/vnd.ms-powerpoint"
	case ".zip":
		return "application/zip"
	default:
		return "application/octet-stream"
	}
}

func detectAudioMimeType(filename string) string {
	ext := strings.ToLower(filepath.Ext(filename))
	switch ext {
	case ".mp3":
		return "audio/mpeg"
	case ".wav":
		return "audio/wav"
	case ".ogg":
		return "audio/ogg"
	case ".webm":
		return "audio/webm"
	case ".m4a":
		return "audio/mp4"
	case ".flac":
		return "audio/flac"
	case ".opus":
		return "audio/opus"
	case ".aac":
		return "audio/aac"
	default:
		return "audio/webm"
	}
}
