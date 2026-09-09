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
	defaultVerdictBatchSize = 5000
	defaultVerdictMaxRows   = 100000
	maxVerdictRows          = 10000000
)

type verdictShadowConfig struct {
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
	cfg, err := loadVerdictShadowConfig()
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(parent, 30*time.Minute)
	defer cancel()

	postgres, err := sql.Open("postgres", cfg.postgresURL)
	if err != nil {
		return fmt.Errorf("open PostgreSQL verdict shadow source: %w", err)
	}
	defer postgres.Close()
	pingCtx, pingCancel := context.WithTimeout(ctx, 5*time.Second)
	defer pingCancel()
	if err := postgres.PingContext(pingCtx); err != nil {
		return fmt.Errorf("ping PostgreSQL verdict shadow source: %w", err)
	}
	clickhouse, err := koscheiclickhouse.NewFromEnv()
	if err != nil {
		return err
	}

	snapshot, err := postgres.BeginTx(ctx, &sql.TxOptions{ReadOnly: true, Isolation: sql.LevelRepeatableRead})
	if err != nil {
		return fmt.Errorf("begin PostgreSQL repeatable-read verdict snapshot: %w", err)
	}
	defer func() { _ = snapshot.Rollback() }()

	var expected int
	if err := snapshot.QueryRowContext(ctx, `
		SELECT count(*)
		FROM security_radar_verdicts
		WHERE updated_at >= $1 AND updated_at < $2
	`, cfg.since, cfg.until).Scan(&expected); err != nil {
		return fmt.Errorf("count PostgreSQL verdict shadow rows: %w", err)
	}
	if expected > cfg.maxRows {
		return fmt.Errorf("verdict shadow window contains %d rows above safety cap %d; narrow KOSCHEI_CLICKHOUSE_VERDICT_SHADOW_SINCE or deliberately raise KOSCHEI_CLICKHOUSE_VERDICT_SHADOW_MAX_ROWS (hard cap %d)", expected, cfg.maxRows, maxVerdictRows)
	}

	rows, err := snapshot.QueryContext(ctx, `
		SELECT
			v.id::text,
			COALESCE(v.event_id::text,''),
			v.module_id,
			v.target,
			v.target_type,
			v.network,
			v.grade,
			v.risk_index,
			v.risk_level,
			v.verdict,
			v.recommendation,
			v.evidence,
			v.signals,
			v.rule_version,
			v.signed,
			COALESCE(v.signature,''),
			COALESCE(v.source,''),
			COALESCE(e.event_type,''),
			COALESCE(NULLIF(v.signals->>'provider',''),v.source,''),
			v.created_at,
			v.updated_at
		FROM security_radar_verdicts v
		LEFT JOIN security_radar_events e ON e.id=v.event_id
		WHERE v.updated_at >= $1 AND v.updated_at < $2
		ORDER BY v.id::text ASC
	`, cfg.since, cfg.until)
	if err != nil {
		return fmt.Errorf("read PostgreSQL verdict shadow rows: %w", err)
	}

	batch := make([]koscheiclickhouse.VerdictSnapshot, 0, cfg.batchSize)
	sourceFingerprint := koscheiclickhouse.NewVerdictParityAccumulator()
	copied := 0
	for rows.Next() {
		var item koscheiclickhouse.VerdictSnapshot
		var evidenceRaw, signalsRaw []byte
		if err := rows.Scan(
			&item.VerdictID,
			&item.EventID,
			&item.ModuleID,
			&item.Target,
			&item.TargetType,
			&item.Network,
			&item.Grade,
			&item.RiskIndex,
			&item.RiskLevel,
			&item.Verdict,
			&item.Recommendation,
			&evidenceRaw,
			&signalsRaw,
			&item.RuleVersion,
			&item.Signed,
			&item.Signature,
			&item.Source,
			&item.EventType,
			&item.Provider,
			&item.CreatedAt,
			&item.SourceUpdatedAt,
		); err != nil {
			_ = rows.Close()
			return fmt.Errorf("scan PostgreSQL verdict shadow row: %w", err)
		}
		if err := json.Unmarshal(evidenceRaw, &item.Evidence); err != nil {
			_ = rows.Close()
			return fmt.Errorf("decode PostgreSQL verdict evidence %s: %w", item.VerdictID, err)
		}
		item.Signals = validVerdictJSON(signalsRaw)
		if err := sourceFingerprint.Add(item); err != nil {
			_ = rows.Close()
			return fmt.Errorf("fingerprint PostgreSQL verdict shadow row %s: %w", item.VerdictID, err)
		}
		batch = append(batch, item)
		if len(batch) >= cfg.batchSize {
			if err := clickhouse.InsertVerdictSnapshots(ctx, batch); err != nil {
				_ = rows.Close()
				return err
			}
			copied += len(batch)
			batch = batch[:0]
		}
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return fmt.Errorf("iterate PostgreSQL verdict shadow rows: %w", err)
	}
	if err := rows.Close(); err != nil {
		return fmt.Errorf("close PostgreSQL verdict shadow rows: %w", err)
	}
	if len(batch) > 0 {
		if err := clickhouse.InsertVerdictSnapshots(ctx, batch); err != nil {
			return err
		}
		copied += len(batch)
	}
	if sourceFingerprint.Count() != uint64(expected) {
		return fmt.Errorf("PostgreSQL verdict snapshot count mismatch counted=%d fingerprinted=%d", expected, sourceFingerprint.Count())
	}
	if err := snapshot.Commit(); err != nil {
		return fmt.Errorf("commit PostgreSQL read-only verdict snapshot: %w", err)
	}

	clickhouseCount, err := clickhouse.CountDistinctVerdictSnapshots(ctx, cfg.since, cfg.until)
	if err != nil {
		return err
	}
	if clickhouseCount != uint64(expected) {
		return fmt.Errorf("ClickHouse verdict shadow count parity mismatch postgres=%d clickhouse_distinct=%d copied_this_run=%d window=[%s,%s)", expected, clickhouseCount, copied, cfg.since.Format(time.RFC3339Nano), cfg.until.Format(time.RFC3339Nano))
	}
	clickhouseFingerprint, clickhouseRows, err := clickhouse.VerdictSnapshotsFingerprint(ctx, cfg.since, cfg.until, uint64(cfg.maxRows))
	if err != nil {
		return err
	}
	if clickhouseRows != uint64(expected) {
		return fmt.Errorf("ClickHouse verdict shadow content row mismatch postgres=%d clickhouse=%d", expected, clickhouseRows)
	}
	if sourceFingerprint.SumHex() != clickhouseFingerprint {
		return fmt.Errorf("ClickHouse verdict shadow content fingerprint mismatch postgres_sha256=%s clickhouse_sha256=%s rows=%d", sourceFingerprint.SumHex(), clickhouseFingerprint, expected)
	}
	log.Printf("ClickHouse verdict shadow parity ok postgres=%d clickhouse_distinct=%d copied_this_run=%d content_sha256=%s window=[%s,%s)", expected, clickhouseCount, copied, clickhouseFingerprint, cfg.since.Format(time.RFC3339Nano), cfg.until.Format(time.RFC3339Nano))
	return nil
}

func loadVerdictShadowConfig() (verdictShadowConfig, error) {
	postgresURL := firstVerdictNonEmpty(strings.TrimSpace(os.Getenv("DATABASE_READ_URL")), strings.TrimSpace(os.Getenv("DATABASE_URL")))
	if postgresURL == "" {
		return verdictShadowConfig{}, fmt.Errorf("DATABASE_READ_URL or DATABASE_URL is required")
	}
	until := time.Now().UTC()
	since := until.Add(-24 * time.Hour)
	if raw := strings.TrimSpace(os.Getenv("KOSCHEI_CLICKHOUSE_VERDICT_SHADOW_SINCE")); raw != "" {
		parsed, err := time.Parse(time.RFC3339Nano, raw)
		if err != nil {
			return verdictShadowConfig{}, fmt.Errorf("parse KOSCHEI_CLICKHOUSE_VERDICT_SHADOW_SINCE: %w", err)
		}
		since = parsed.UTC()
	}
	if !since.Before(until) {
		return verdictShadowConfig{}, fmt.Errorf("KOSCHEI_CLICKHOUSE_VERDICT_SHADOW_SINCE must be before now")
	}
	return verdictShadowConfig{
		postgresURL: postgresURL,
		since:       since,
		until:       until,
		batchSize:   boundedVerdictEnvInt("KOSCHEI_CLICKHOUSE_VERDICT_SHADOW_BATCH_SIZE", defaultVerdictBatchSize, 100, 100000),
		maxRows:     boundedVerdictEnvInt("KOSCHEI_CLICKHOUSE_VERDICT_SHADOW_MAX_ROWS", defaultVerdictMaxRows, 1, maxVerdictRows),
	}, nil
}

func boundedVerdictEnvInt(key string, fallback, minValue, maxValue int) int {
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

func validVerdictJSON(value []byte) json.RawMessage {
	if len(value) == 0 || !json.Valid(value) {
		return json.RawMessage(`{}`)
	}
	return json.RawMessage(append([]byte(nil), value...))
}

func firstVerdictNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
