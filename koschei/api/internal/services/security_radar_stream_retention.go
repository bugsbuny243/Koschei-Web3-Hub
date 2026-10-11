package services

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// StreamRetentionPolicy is deliberately opt-in. The default never deletes data.
type StreamRetentionPolicy struct {
	Enabled bool
	Days    int
}

func securityRadarStreamRetentionPolicy() StreamRetentionPolicy {
	days := 14
	if raw := strings.TrimSpace(os.Getenv("KOSCHEI_STREAM_RETENTION_DAYS")); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed >= 1 && parsed <= 3650 {
			days = parsed
		}
	}
	return StreamRetentionPolicy{
		Enabled: envBool("KOSCHEI_STREAM_RETENTION_ENABLED") && envBool("KOSCHEI_STREAM_RETENTION_PARITY_APPROVED"),
		Days:    days,
	}
}

// RunSecurityRadarStreamRetention deletes only decoded hints after an explicit
// opt-in. It must be invoked from the designated worker, never an API request.
// Operators must complete and verify the parity/backup audit before enabling it.\n// Both KOSCHEI_STREAM_RETENTION_ENABLED and\n// KOSCHEI_STREAM_RETENTION_PARITY_APPROVED must be explicitly set.
func RunSecurityRadarStreamRetention(ctx context.Context, db *sql.DB) (int64, error) {
	policy := securityRadarStreamRetentionPolicy()
	if !policy.Enabled {
		return 0, nil
	}
	if db == nil {
		return 0, fmt.Errorf("stream retention requires a database")
	}
	cutoff := time.Now().UTC().AddDate(0, 0, -policy.Days)
	result, err := db.ExecContext(ctx, `
		DELETE FROM security_radar_stream_events
		WHERE id IN (
			SELECT id FROM security_radar_stream_events
			WHERE evidence_quality = 'decoded_stream_hint'
			  AND created_at < $1
			ORDER BY created_at
			LIMIT 1000
		)`, cutoff)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}
