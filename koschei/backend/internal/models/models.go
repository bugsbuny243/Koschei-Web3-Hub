package models

import "time"

type User struct {
	ID        string    `json:"id"`
	Email     string    `json:"email"`
	Tier      string    `json:"tier"`
	Credits   int       `json:"credits"`
	CreatedAt time.Time `json:"created_at"`
}

type AITextRequest struct {
	Message string `json:"message" binding:"required,min=2"`
}
type ImageRequest struct {
	Prompt string `json:"prompt" binding:"required,min=2"`
}
type ImageEditRequest struct {
	Prompt   string `json:"prompt" binding:"required,min=2"`
	ImageURL string `json:"image_url" binding:"required,url"`
}
type TTSRequest struct {
	Text  string `json:"text" binding:"required,min=2"`
	Voice string `json:"voice"`
}
type STTRequest struct {
	AudioURL string `json:"audio_url" binding:"required,url"`
}
type VideoRequest struct {
	Prompt      string `json:"prompt" binding:"required,min=2"`
	DurationSec int    `json:"duration_sec" binding:"omitempty,min=1,max=30"`
}
