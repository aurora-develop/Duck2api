package initialize

import (
	duckgoConvert "aurora/conversion/requests/duckgo"
	"aurora/httpclient/bogdanfinn"
	"aurora/internal/duckgo"
	"aurora/internal/proxys"
	officialtypes "aurora/typings/official"

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
	proxyUrl := h.proxy.GetProxyIP()
	client := bogdanfinn.NewStdClient()
	token, err := duckgo.InitXVQD(client, proxyUrl)
	if err != nil {
		c.JSON(500, gin.H{
			"error": err.Error(),
		})
		return
	}

	translated_request := duckgoConvert.ConvertAPIRequest(original_request)
	response, err := duckgo.POSTconversation(client, translated_request, token, proxyUrl)
	if err != nil {
		c.JSON(500, gin.H{
			"error": "request conversion error",
		})
		return
	}

	defer response.Body.Close()
	if duckgo.Handle_request_error(c, response) {
		return
	}
	var response_part string
	response_part = duckgo.Handler(c, response, translated_request, original_request.Stream)
	if c.Writer.Status() != 200 {
		return
	}
	if !original_request.Stream {
		c.JSON(200, officialtypes.NewChatCompletionWithModel(response_part, translated_request.Model))
	} else {
		c.String(200, "data: [DONE]\n\n")
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
		"o3-mini",
		"gpt-3.5-turbo-0125",
		"claude-3-haiku-20240307",
		"meta-llama/Llama-3.3-70B-Instruct-Turbo",
		"mistralai/Mixtral-8x7B-Instruct-v0.1",
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
