package router

import (
	"koschei/backend/internal/handlers"

	"github.com/gin-gonic/gin"
)

func New(h handlers.Handler) *gin.Engine {
	r := gin.Default()
	r.GET("/health", func(c *gin.Context) { c.JSON(200, gin.H{"ok": true}) })
	runtime := r.Group("/api/runtime")
	{
		runtime.POST("/projects", h.CreateRuntimeProject)
		runtime.GET("/projects", h.ListRuntimeProjects)
		runtime.GET("/projects/:id", h.GetRuntimeProject)
		runtime.GET("/tasks", h.ListRuntimeTasks)
		runtime.GET("/tasks/:id", h.GetRuntimeTask)
		runtime.GET("/logs/:projectId", h.GetRuntimeLogs)
	}
	owner := r.Group("/api/owner/runtime")
	{
		owner.POST("/tasks/:id/retry", h.RetryTask)
		owner.POST("/tasks/:id/cancel", h.CancelTask)
		owner.PATCH("/tasks/:id/status", h.PatchTaskStatus)
	}
	api := r.Group("/api/ai")
	{
		api.POST("/chat", h.Chat)
		api.POST("/code", h.Code)
		api.POST("/reason", h.Reason)
		api.POST("/image", h.Image)
		api.POST("/image-edit", h.ImageEdit)
		api.POST("/video", h.Video)
		api.POST("/cinema", h.Cinema)
		api.POST("/tts", h.TTS)
		api.POST("/stt", h.STT)
	}
	return r
}
