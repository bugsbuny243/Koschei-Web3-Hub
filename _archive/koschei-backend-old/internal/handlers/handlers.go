package handlers

import (
	"database/sql"
	"encoding/json"
	"net/http"

	"koschei/backend/internal/models"
	"koschei/backend/internal/services"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type Handler struct {
	DB              *sql.DB
	JWTSecret       string
	Router          services.AIRouter
	Together        services.TogetherClient
	Worker          services.PythonWorkerClient
	OwnerGodModeKey string
}

func (h Handler) Register(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, gin.H{"error": "not in scope"})
}

func (h Handler) enforceAccess(c *gin.Context, task services.TaskType) bool {
	mode := c.GetHeader("X-Access-Mode")
	if mode == "owner_god_mode" {
		if h.OwnerGodModeKey == "" || c.GetHeader("X-Owner-Key") != h.OwnerGodModeKey {
			c.JSON(http.StatusForbidden, gin.H{"error": "invalid owner key"})
			return false
		}
		return true
	}
	if task == services.TaskCinema || task == services.TaskVideo {
		c.JSON(http.StatusForbidden, gin.H{"error": "public_saas cannot access premium video routes"})
		return false
	}
	return true
}

func (h Handler) handleText(c *gin.Context, task services.TaskType) {
	if !h.enforceAccess(c, task) {
		return
	}
	var req models.AITextRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if !h.enforceAccess(c, task) {
		return
	}
	model := h.Router.SelectModel(task)
	resp, err := h.Together.Chat(model, req.Message)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	h.trackGeneration(c, string(task), model, "text", 1, map[string]any{"response": resp})
	c.JSON(http.StatusOK, gin.H{"model": model, "response": resp})
}

func (h Handler) Chat(c *gin.Context)   { h.handleText(c, services.TaskChat) }
func (h Handler) Code(c *gin.Context)   { h.handleText(c, services.TaskCode) }
func (h Handler) Reason(c *gin.Context) { h.handleText(c, services.TaskReason) }

func (h Handler) Image(c *gin.Context) {
	var req models.ImageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	h.mediaTask(c, services.TaskImage, map[string]any{"prompt": req.Prompt})
}
func (h Handler) ImageEdit(c *gin.Context) {
	var req models.ImageEditRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	h.mediaTask(c, services.TaskImageEdit, map[string]any{"prompt": req.Prompt, "image_url": req.ImageURL})
}
func (h Handler) Video(c *gin.Context) {
	var req models.VideoRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	h.mediaTask(c, services.TaskVideo, map[string]any{"prompt": req.Prompt, "duration_sec": req.DurationSec})
}
func (h Handler) Cinema(c *gin.Context) {
	var req models.VideoRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	h.mediaTask(c, services.TaskCinema, map[string]any{"prompt": req.Prompt, "duration_sec": req.DurationSec})
}
func (h Handler) TTS(c *gin.Context) {
	var req models.TTSRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	h.mediaTask(c, services.TaskTTS, map[string]any{"text": req.Text, "voice": req.Voice})
}
func (h Handler) STT(c *gin.Context) {
	var req models.STTRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	h.mediaTask(c, services.TaskSTT, map[string]any{"audio_url": req.AudioURL})
}

func (h Handler) mediaTask(c *gin.Context, task services.TaskType, input map[string]any) {
	model := h.Router.SelectModel(task)
	res, err := h.Worker.Generate(task, model, input)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	h.trackGeneration(c, string(task), model, "media", 5, res)
	c.JSON(http.StatusOK, gin.H{"model": model, "result": res})
}

func (h Handler) trackGeneration(c *gin.Context, task, model, outputType string, credits int, payload map[string]any) {
	userID := c.GetHeader("X-User-ID")
	if userID == "" {
		userID = uuid.Nil.String()
	}
	_, _ = h.DB.Exec(`INSERT INTO ai_generations (id,user_id,task_type,model_name,output_type,credits_used,response_payload) VALUES ($1,$2,$3,$4,$5,$6,$7::jsonb)`, uuid.NewString(), userID, task, model, outputType, credits, toJSON(payload))
	if userID != uuid.Nil.String() {
		_, _ = h.DB.Exec(`UPDATE users SET credits = GREATEST(credits - $1, 0) WHERE id=$2`, credits, userID)
	}
}

func toJSON(v any) string {
	b, _ := json.Marshal(v)
	return string(b)
}
