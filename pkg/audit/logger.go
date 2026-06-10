package audit

import (
	"context"
	"database/sql"
	"time"

	"github.com/bugsbuny243/Koschei-Web3-Hub/koschei/api/internal/db"
)

var dbConn *sql.DB

func Init(c *sql.DB) {
	dbConn = c
}

type Event struct {
	Action     string                 `json:"action"`
	Email      string                 `json:"email"`
	IP         string                 `json:"ip"`
	Metadata   map[string]interface{} `json:"metadata"`
	Timestamp  time.Time              `json:"timestamp"`
}

func Log(ctx context.Context, e Event) {
	if dbConn == nil {
		return
	}
	_, _ = dbConn.ExecContext(ctx,
		`INSERT INTO admin_audit_logs (admin_email, action, metadata, created_at)
		 VALUES ($1, $2, $3, $4)`,
		e.Email, e.Action, e.Metadata, e.Timestamp)
}
