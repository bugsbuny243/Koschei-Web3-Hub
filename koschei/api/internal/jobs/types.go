package jobs

import "time"

const (
	StatusQueued    = "queued"
	StatusRunning   = "running"
	StatusCompleted = "completed"
	StatusFailed    = "failed"
)

type Job struct {
	ID             string    `json:"id"`
	UserID         string    `json:"user_id,omitempty"`
	Email          string    `json:"email,omitempty"`
	Type           string    `json:"job_type"`
	Status         string    `json:"status"`
	Network        string    `json:"network,omitempty"`
	Target         string    `json:"target,omitempty"`
	RequestPayload []byte    `json:"-"`
	ResultPayload  []byte    `json:"-"`
	ErrorCode      string    `json:"error_code,omitempty"`
	ErrorMessage   string    `json:"error_message,omitempty"`
	Progress       int       `json:"progress"`
	Attempts       int       `json:"attempts"`
	QueuedAt       time.Time `json:"queued_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

type CreateInput struct {
	UserID, Email, Type, Network, Target string
	Request                              any
}

type Queue interface {
	Publish(job Job) error
	Close() error
}
