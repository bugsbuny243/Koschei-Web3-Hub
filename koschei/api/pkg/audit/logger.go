package audit

import (
	"context"
	"database/sql"
	"time"
)

var dbConn *sql.DB

// Init initializes the audit logger with database connection
func Init(database *sql.DB) {
	dbConn = database
}

// Event represents an audit log entry
type Event struct {
	Action    string                 `json:"action"`
	Email     string                 `json:"email"`
	IP        string                 `json:"ip"`
	Metadata  map[string]interface{} `json:"metadata"`
	Timestamp time.Time              `json:"timestamp"`
}

// Log writes an event to admin_audit_logs table
func Log(ctx context.Context, e Event) {
	if dbConn == nil {
		return
	}

	const query = `
	INSERT INTO admin_audit_logs 
	(admin_email, action, metadata, created_at) 
	VALUES ($1, $2, $3, $4)`

	_, _ = dbConn.ExecContext(ctx, query,
		e.Email, e.Action, e.Metadata, e.Timestamp)
}
