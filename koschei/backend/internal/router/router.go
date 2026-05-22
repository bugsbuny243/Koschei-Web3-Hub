package router

import (
	"koschei/backend/internal/handlers"

	"github.com/gin-gonic/gin"
)

func New(h handlers.Handler) *gin.Engine {
	r := gin.Default()
	r.GET("/health", func(c *gin.Context) { c.JSON(200, gin.H{"ok": true}) })
	api := r.Group("/api")
	{
		api.POST("/auth/register", h.Register)
		api.POST("/chat", h.Chat)
		api.POST("/generate/image", h.GenerateImage)
		api.POST("/generate/video", h.GenerateVideo)
	}
	return r
}
