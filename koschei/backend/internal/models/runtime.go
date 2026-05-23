package models

import "time"

type RuntimeProject struct {
	ID        string    `json:"id"`
	Email     string    `json:"email"`
	Title     string    `json:"title"`
	Prompt    string    `json:"prompt"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type RuntimeTask struct {
	ID          string     `json:"id"`
	ProjectID   string     `json:"project_id"`
	Email       string     `json:"email"`
	TaskType    string     `json:"task_type"`
	Tool        string     `json:"tool"`
	Prompt      string     `json:"prompt"`
	Status      string     `json:"status"`
	Priority    int        `json:"priority"`
	Result      *string    `json:"result,omitempty"`
	Error       *string    `json:"error,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	StartedAt   *time.Time `json:"started_at,omitempty"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

type RuntimeLog struct {
	ID        string    `json:"id"`
	ProjectID string    `json:"project_id"`
	TaskID    *string   `json:"task_id,omitempty"`
	Level     string    `json:"level"`
	Message   string    `json:"message"`
	CreatedAt time.Time `json:"created_at"`
}

type CreateRuntimeProjectRequest struct {
	Email  string `json:"email" binding:"required,email"`
	Title  string `json:"title" binding:"required,min=2"`
	Prompt string `json:"prompt" binding:"required,min=2"`
}
