package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	_ "github.com/lib/pq"

	koscheiclickhouse "koschei/api/internal/clickhouse"
)

const (
	defaultBatchSize = 10000
	defaultMaxRows   = 100000
	maxAllowedRows   = 10000000
)

type config struct {
	postgresURL string
	since       time.Time
	until       time.Time
	batchSize   int
	maxRows     int
}

func main() {
	if err := run(context.Background()); err != nil {
		log.Fatal(err)
	}
}

func run(parent context.Context) error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(parent, 30*time.Minute)
	defer cancel()

	postgres, err := sql.Open("postgres", cfg.postgresURL)
	if err != nil {
		return fmt.Errorf("open PostgreSQL shadow source: %w", err)
	}
	defer postgres.Close()

	pingCtx, pingCancel := context.WithTimeout(ctx, 5*time.Second)
	defer pingCancel()
	if err := postgres.PingContext(pingCtx); err != nil {
		return fmt.Errorf("ping PostgreSQL shadow source: %w", err)
	}

	clickhouse, err := koscheiclickhouse.NewFromEnv()
	if err != nil {
		return err
	}

	snapshot, err := postgres.BeginTx(ctx, &sql.TxOptions{ReadOnly: true, Isolation: sql.LevelRepeatableRead})
	if err != nil {
		return fmt.Errorf("begin PostgreSQL repeatable-read shadow snapshot: %w", err)
	}
	defer func() { _ = snapshot.Rollback() }()

	var expected int
	if err := snapshot.QueryRowContext(ctx, `
		SELECT count(*)
		FROM security_radar_stream_events
		WHERE created_at >= $1 AND created_at < $2
	`, cfg.since, cfg.until).Scan(&expected); err != nil {
		return fmt.Errorf("count PostgreSQL shadow rows: %w", err)
	}
	if expected > cfg.maxRows {
		return fmt.Errorf(
			"shadow window contains %d rows, above safety cap %d; narrow KOSCHEI_CLICKHOUSE_SHADOW_SINCE or deliberately raise KOSCHEI_CLICKHOUSE_SHADOW_MAX_ROWS (hard cap %d)",
			expected,
			cfg.maxRows,
			maxAllowedRows,
		)
	}

	rows, err := snapshot.QueryContext(ctx, `
		SELECT
			id::text,
			COALESCE(provider,''),
			COALESCE(stream_mode,''),
			COALESCE(network,''),
			COALESCE(module_id,''),
			COALESCE(event_type,''),
			COALESCE(target,''),
			COALESCE(target_type,''),
			COALESCE(signature,''),
			COALESCE(slot,0),
			COALESCE(program_id,''),
			COALESCE(evidence_quality,''),
			decoded,
			raw_event,
			created_at
		FROM security_radar_stream_events
		WHERE created_at >= $1 AND created_at < $2
		ORDER BY id::text ASC
	`, cfg.since, cfg.until)
	if err != nil {
		return fmt.Errorf("read PostgreSQL shadow rows: %w", err)
	}

	batch := make([]koscheiclickhouse.StreamEvent, 0, cfg.batchSize)
	sourceFingerprint := koscheiclickhouse.NewStreamParityAccumulator()
	copied := 0
	for rows.Next() {
		var event koscheiclickhouse.StreamEvent
		var slot int64
		var decoded, rawEvent []byte
		if err := rows.Scan(
			&event.EventID,
			&event.Provider,
			&event.StreamMode,
			&event.Network,
			&event.ModuleID,
			&event.EventType,
			&event.Target,
			&event.TargetType,
			&event.Signature,
			&slot,
			&event.ProgramID,
			&event.EvidenceQuality,
			&decoded,
			&rawEvent,
			&event.CreatedAt,
		); err != nil {
			_ = rows.Close()
			return fmt.Errorf("scan PostgreSQL shadow row: %w", err)
		}
		if slot > 0 {
			event.Slot = uint64(slot)
		}
		event.Decoded = validRawJSON(decoded)
		event.RawEvent = validRawJSON(rawEvent)
		if err := sourceFingerprint.Add(event); err != nil {
			_ = rows.Close()
			return fmt.Errorf("fingerprint PostgreSQL shadow row %s: %w", event.EventID, err)
		}
		batch = append(batch, event)

		if len(batch) >= cfg.batchSize {
			if err := clickhouse.InsertStreamEvents(ctx, batch); err != nil {
				_ = rows.Close()
				return err
			}
			copied += len(batch)
			batch = batch[:0]
		}
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return fmt.Errorf("iterate PostgreSQL shadow rows: %w", err)
	}
	if err := rows.Close(); err != nil {
		return fmt.Errorf("close PostgreSQL shadow rows: %w", err)
	}
	if len(batch) > 0 {
		if err := clickhouse.InsertStreamEvents(ctx, batch); err != nil {
			return err
		}
		copied += len(batch)
	}

	if sourceFingerprint.Count() != uint64(expected) {
		return fmt.Errorf("PostgreSQL repeatable-read shadow snapshot count mismatch: counted=%d fingerprinted=%d", expected, sourceFingerprint.Count())
	}
	if err := snapshot.Commit(); err != nil {
		return fmt.Errorf("commit PostgreSQL read-only shadow snapshot: %w", err)
	}

	clickhouseCount, err := clickhouse.CountDistinctStreamEvents(ctx, cfg.since, cfg.until)
	if err != nil {
		return err
	}
	if uint64(expected) != clickhouseCount {
		return fmt.Errorf(
			"ClickHouse shadow count parity mismatch: postgres=%d clickhouse_distinct=%d copied_this_run=%d window=[%s,%s)",
			expected,
			clickhouseCount,
			copied,
			cfg.since.Format(time.RFC3339),
			cfg.until.Format(time.RFC3339),
		)
	}

	clickhouseFingerprint, clickhouseRows, err := clickhouse.StreamEventsFingerprint(ctx, cfg.since, cfg.until, uint64(cfg.maxRows))
	if err != nil {
		return err
	}
	if clickhouseRows != uint64(expected) {
		return fmt.Errorf(
			"ClickHouse shadow content row mismatch: postgres=%d clickhouse=%d window=[%s,%s)",
			expected,
			clickhouseRows,
			cfg.since.Format(time.RFC3339),
			cfg.until.Format(time.RFC3339),
		)
	}
	if sourceFingerprint.SumHex() != clickhouseFingerprint {
		return fmt.Errorf(
			"ClickHouse shadow content fingerprint mismatch: postgres_sha256=%s clickhouse_sha256=%s rows=%d window=[%s,%s)",
			sourceFingerprint.SumHex(),
			clickhouseFingerprint,
			expected,
			cfg.since.Format(time.RFC3339),
			cfg.until.Format(time.RFC3339),
		)
	}

	log.Printf(
		"ClickHouse shadow parity ok postgres=%d clickhouse_distinct=%d copied_this_run=%d content_sha256=%s window=[%s,%s)",
		expected,
		clickhouseCount,
		copied,
		clickhouseFingerprint,
		cfg.since.Format(time.RFC3339),
		cfg.until.Format(time.RFC3339),
	)
	return nil
}

func loadConfig() (config, error) {
	postgresURL := firstNonEmpty(strings.TrimSpace(os.Getenv("DATABASE_READ_URL")), strings.TrimSpace(os.Getenv("DATABASE_URL")))
	if postgresURL == "" {
		return config{}, fmt.Errorf("DATABASE_READ_URL or DATABASE_URL is required")
	}

	until := time.Now().UTC()
	since := until.Add(-24 * time.Hour)
	if raw := strings.TrimSpace(os.Getenv("KOSCHEI_CLICKHOUSE_SHADOW_SINCE")); raw != "" {
		parsed, err := time.Parse(time.RFC3339, raw)
		if err != nil {
			return config{}, fmt.Errorf("parse KOSCHEI_CLICKHOUSE_SHADOW_SINCE as RFC3339: %w", err)
		}
		since = parsed.UTC()
	}
	if !since.Before(until) {
		return config{}, fmt.Errorf("KOSCHEI_CLICKHOUSE_SHADOW_SINCE must be before now")
	}

	batchSize := boundedEnvInt("KOSCHEI_CLICKHOUSE_SHADOW_BATCH_SIZE", defaultBatchSize, 1000, 100000)
	maxRows := boundedEnvInt("KOSCHEI_CLICKHOUSE_SHADOW_MAX_ROWS", defaultMaxRows, 1, maxAllowedRows)
	return config{
		postgresURL: postgresURL,
		since:       since,
		until:       until,
		batchSize:   batchSize,
		maxRows:     maxRows,
	}, nil
}

func boundedEnvInt(key string, fallback, minValue, maxValue int) int {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(raw)
	if err != nil || parsed < minValue || parsed > maxValue {
		return fallback
	}
	return parsed
}

func validRawJSON(value []byte) json.RawMessage {
	if len(value) == 0 || !json.Valid(value) {
		return json.RawMessage(`{}`)
	}
	copyValue := append([]byte(nil), value...)
	return json.RawMessage(copyValue)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
