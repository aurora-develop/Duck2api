package initialize

import (
	"aurora/middlewares"
	"os"

	"github.com/gin-gonic/gin"
)

func RegisterRouter() *gin.Engine {
	handler := NewHandle(
		checkProxy(),
	)

	router := gin.Default()
	router.Use(middlewares.Cors)

	router.GET("/", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"message": "Hello, world!",
		})
	})

	router.GET("/ping", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"message": "pong",
		})
	})

	prefixGroup := os.Getenv("PREFIX")
	if prefixGroup != "" {
		prefixRouter := router.Group(prefixGroup)
		{
			registerRoutes(prefixRouter, handler)
		}
	}

	registerRoutes(&router.RouterGroup, handler)
	return router
}

func registerRoutes(group *gin.RouterGroup, handler *Handler) {
	// Chat completions
	group.OPTIONS("/v1/chat/completions", optionsHandler)
	group.POST("/v1/chat/completions", middlewares.Authorization, handler.duckduckgo)

	// Responses API
	group.OPTIONS("/v1/responses", optionsHandler)
	group.OPTIONS("/v1/response", optionsHandler)
	group.POST("/v1/responses", middlewares.Authorization, handler.responses)
	group.POST("/v1/response", middlewares.Authorization, handler.responses)

	// Images
	group.OPTIONS("/v1/images/generations", optionsHandler)
	group.OPTIONS("/v1/images/edits", optionsHandler)
	group.POST("/v1/images/generations", middlewares.Authorization, handler.imageGenerations)
	group.POST("/v1/images/edits", middlewares.Authorization, handler.imageEdits)

	// Files
	group.OPTIONS("/v1/files", optionsHandler)
	group.POST("/v1/files", middlewares.Authorization, handler.filesUpload)
	group.GET("/v1/files", middlewares.Authorization, handler.filesList)
	group.GET("/v1/files/:file_id", middlewares.Authorization, handler.filesGet)
	group.DELETE("/v1/files/:file_id", middlewares.Authorization, handler.filesDelete)
	group.GET("/v1/files/:file_id/content", middlewares.Authorization, handler.filesContent)

	// Audio
	group.OPTIONS("/v1/audio/transcriptions", optionsHandler)
	group.POST("/v1/audio/transcriptions", middlewares.Authorization, handler.audioTranscriptions)
	group.OPTIONS("/v1/audio/speech", optionsHandler)
	group.POST("/v1/audio/speech", middlewares.Authorization, handler.audioSpeech)

	// Models
	group.OPTIONS("/v1/models", optionsHandler)
	group.GET("/v1/models", middlewares.Authorization, handler.engines)
}
