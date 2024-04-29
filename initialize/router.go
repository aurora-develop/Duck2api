package initialize

import (
	"aurora/middlewares"

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

	router.OPTIONS("/v1/chat/completions", optionsHandler)
	router.OPTIONS("/v1/chat/models", optionsHandler)
	authGroup := router.Group("").Use(middlewares.Authorization)
	authGroup.POST("/v1/chat/completions", handler.duckduckgo)
	authGroup.GET("/v1/models", handler.engines)
	return router
}
