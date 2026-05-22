package models

import "time"

type User struct {
	ID        string    `json:"id"`
	Email     string    `json:"email"`
	Password  string    `json:"-"`
	Tier      string    `json:"tier"`
	Credits   int       `json:"credits"`
	CreatedAt time.Time `json:"created_at"`
}

type ChatRequest struct {
	Message  string `json:"message" binding:"required"`
	ImageURL string `json:"image_url,omitempty"`
}

type GenerationRequest struct {
	Prompt string `json:"prompt" binding:"required"`
}
