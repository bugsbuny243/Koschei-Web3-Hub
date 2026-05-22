package handlers

import (
	"database/sql"
	"net/http"

	"koschei/backend/internal/models"
	"koschei/backend/internal/services"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type Handler struct {
	DB        *sql.DB
	JWTSecret string
	Router    services.AIRouter
	Together  services.TogetherClient
	Fal       services.FalClient
}

func (h Handler) Register(c *gin.Context) {
	var req struct{ Email, Password string }
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	hash, _ := services.HashPassword(req.Password)
	id := uuid.NewString()
	_, err := h.DB.Exec(`INSERT INTO users (id,email,password_hash,tier,credits) VALUES ($1,$2,$3,'free',100)`, id, req.Email, hash)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	token, _ := services.CreateJWT(h.JWTSecret, id, req.Email, "free")
	c.JSON(http.StatusCreated, gin.H{"token": token})
}

func (h Handler) Chat(c *gin.Context) {
	var req models.ChatRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	model := h.Router.SelectModel(req.Message, req.ImageURL != "")
	response, err := h.Together.Chat(model, req.Message)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"model": model, "response": response})
}

func (h Handler) GenerateImage(c *gin.Context) {
	var req models.GenerationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	result, err := h.Fal.GenerateImage(req.Prompt)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"result": result})
}

func (h Handler) GenerateVideo(c *gin.Context) {
	var req models.GenerationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	result, err := h.Fal.GenerateVideo(req.Prompt)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"result": result})
}
