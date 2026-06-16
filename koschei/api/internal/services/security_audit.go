package services

import (
	"context"
	"database/sql"
	"encoding/json"
	"log"
	"time"
)

type SecurityAuditEvent struct {
	EventType string         `json:"event_type"`
	ActorType string         `json:"actor_type"`
	ActorID   string         `json:"actor_id,omitempty"`
	IP        string         `json:"ip,omitempty"`
	UserAgent string         `json:"user_agent,omitempty"`
	Path      string         `json:"path,omitempty"`
	Severity  string         `json:"severity"`
	Metadata  map[string]any `json:"metadata,omitempty"`
	CreatedAt time.Time      `json:"created_at,omitempty"`
}

func WriteSecurityAuditEvent(ctx context.Context, db *sql.DB, ev SecurityAuditEvent) {
	if db == nil {
		return
	}
	if ev.EventType == "" {
		return
	}
	if ev.Severity == "" {
		ev.Severity = "info"
	}
	metadata, _ := json.Marshal(sanitizeSecurityAuditMetadata(ev.Metadata))
	_, err := db.ExecContext(ctx, `
		INSERT INTO security_audit_events (event_type,actor_type,actor_id,ip,user_agent,path,severity,metadata,created_at)
		VALUES ($1,$2,NULLIF($3,''),NULLIF($4,''),NULLIF($5,''),NULLIF($6,''),$7,$8::jsonb,now())`, ev.EventType, ev.ActorType, ev.ActorID, ev.IP, ev.UserAgent, ev.Path, ev.Severity, string(metadata))
	if err != nil {
		log.Printf("security audit write failed")
	}
}

func LatestSecurityAuditEvents(ctx context.Context, db *sql.DB, limit int) ([]SecurityAuditEvent, error) {
	if db == nil {
		return []SecurityAuditEvent{}, nil
	}
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}
	rows, err := db.QueryContext(ctx, `
		SELECT event_type, COALESCE(actor_type,''), COALESCE(actor_id,''), COALESCE(ip,''), COALESCE(user_agent,''), COALESCE(path,''), COALESCE(severity,'info'), COALESCE(metadata,'{}'::jsonb), created_at
		FROM security_audit_events
		ORDER BY created_at DESC
		LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []SecurityAuditEvent{}
	for rows.Next() {
		var ev SecurityAuditEvent
		var raw []byte
		if err := rows.Scan(&ev.EventType, &ev.ActorType, &ev.ActorID, &ev.IP, &ev.UserAgent, &ev.Path, &ev.Severity, &raw, &ev.CreatedAt); err != nil {
			return nil, err
		}
		_ = json.Unmarshal(raw, &ev.Metadata)
		if ev.Metadata == nil {
			ev.Metadata = map[string]any{}
		}
		items = append(items, ev)
	}
	return items, rows.Err()
}

func sanitizeSecurityAuditMetadata(metadata map[string]any) map[string]any {
	if metadata == nil {
		return map[string]any{}
	}
	out := map[string]any{}
	for key, value := range metadata {
		if IsSensitiveEnvName(key) {
			out[key] = "redacted"
			continue
		}
		out[key] = value
	}
	return out
}
