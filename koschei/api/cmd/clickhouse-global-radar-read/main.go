package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	clickhouse "koschei/api/internal/clickhouse"
)

const globalRadarMemoryReadSchemaVersion = "koschei.global-radar-memory-read.v1"

type globalRadarMemoryReadOutput struct {
	SchemaVersion string                               `json:"schema_version"`
	Network       string                               `json:"network"`
	SubjectID     string                               `json:"subject_id,omitempty"`
	RecordType    string                               `json:"record_type,omitempty"`
	Since         time.Time                            `json:"since"`
	Until         time.Time                            `json:"until"`
	RecordCount   int                                  `json:"record_count"`
	Records       []clickhouse.GlobalRadarStoredRecord `json:"records"`
	RiskScore     bool                                 `json:"risk_score_produced"`
	Limitations   []string                             `json:"limitations"`
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "clickhouse global radar read failed:", err)
		os.Exit(1)
	}
}

func run() error {
	now := time.Now().UTC()
	networkDefault := strings.TrimSpace(os.Getenv("KOSCHEI_GLOBAL_RADAR_NETWORK"))
	subjectDefault := strings.TrimSpace(os.Getenv("KOSCHEI_GLOBAL_RADAR_SUBJECT_ID"))
	recordTypeDefault := strings.TrimSpace(os.Getenv("KOSCHEI_GLOBAL_RADAR_RECORD_TYPE"))
	limitDefault := uint64(500)
	if raw := strings.TrimSpace(os.Getenv("KOSCHEI_GLOBAL_RADAR_LIMIT")); raw != "" {
		if parsed, err := strconv.ParseUint(raw, 10, 64); err == nil {
			limitDefault = parsed
		}
	}

	network := flag.String("network", networkDefault, "exact network label; required")
	subjectID := flag.String("subject-id", subjectDefault, "optional exact canonical Global Radar subject id")
	recordType := flag.String("record-type", recordTypeDefault, "optional observation, relation, bridge_link, or verdict_reference")
	sinceRaw := flag.String("since", strings.TrimSpace(os.Getenv("KOSCHEI_GLOBAL_RADAR_SINCE")), "RFC3339 lower bound; defaults to 24h before until")
	untilRaw := flag.String("until", strings.TrimSpace(os.Getenv("KOSCHEI_GLOBAL_RADAR_UNTIL")), "RFC3339 upper bound; defaults to now")
	limit := flag.Uint64("limit", limitDefault, "maximum Global Radar graph rows")
	flag.Parse()

	until := now
	var err error
	if strings.TrimSpace(*untilRaw) != "" {
		until, err = time.Parse(time.RFC3339Nano, strings.TrimSpace(*untilRaw))
		if err != nil {
			return fmt.Errorf("parse until: %w", err)
		}
	}
	since := until.Add(-24 * time.Hour)
	if strings.TrimSpace(*sinceRaw) != "" {
		since, err = time.Parse(time.RFC3339Nano, strings.TrimSpace(*sinceRaw))
		if err != nil {
			return fmt.Errorf("parse since: %w", err)
		}
	}

	client, err := clickhouse.NewFromEnv()
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	records, err := client.ReadGlobalRadarGraph(ctx, clickhouse.GlobalRadarGraphReadRequest{
		Network:    strings.TrimSpace(*network),
		SubjectID:  strings.TrimSpace(*subjectID),
		RecordType: strings.ToLower(strings.TrimSpace(*recordType)),
		Since:      since,
		Until:      until,
		Limit:      *limit,
	})
	if err != nil {
		return err
	}

	output := globalRadarMemoryReadOutput{
		SchemaVersion: globalRadarMemoryReadSchemaVersion,
		Network:       strings.TrimSpace(*network),
		SubjectID:     strings.TrimSpace(*subjectID),
		RecordType:    strings.ToLower(strings.TrimSpace(*recordType)),
		Since:         since.UTC(),
		Until:         until.UTC(),
		RecordCount:   len(records),
		Records:       records,
		RiskScore:     false,
		Limitations: []string{
			"This is a bounded read-only ClickHouse Global Radar memory view; it does not create or change a security verdict.",
			"Stored payload SHA-256 is rechecked during reads before records are returned.",
			"Cross-network relationships remain only as strong as their persisted explicit evidence references; co-occurrence is not attribution.",
		},
	}

	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(output); err != nil {
		return fmt.Errorf("encode Global Radar memory output: %w", err)
	}
	return nil
}
