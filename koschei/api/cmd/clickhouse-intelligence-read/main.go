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

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "clickhouse intelligence read failed:", err)
		os.Exit(1)
	}
}

func run() error {
	now := time.Now().UTC()
	targetDefault := strings.TrimSpace(os.Getenv("KOSCHEI_CLICKHOUSE_INTELLIGENCE_TARGET"))
	networkDefault := strings.TrimSpace(os.Getenv("KOSCHEI_CLICKHOUSE_INTELLIGENCE_NETWORK"))
	if networkDefault == "" {
		networkDefault = "solana-mainnet"
	}
	limitDefault := uint64(200)
	if raw := strings.TrimSpace(os.Getenv("KOSCHEI_CLICKHOUSE_INTELLIGENCE_LIMIT")); raw != "" {
		if parsed, err := strconv.ParseUint(raw, 10, 64); err == nil {
			limitDefault = parsed
		}
	}
	outputDefault := strings.ToLower(strings.TrimSpace(os.Getenv("KOSCHEI_CLICKHOUSE_INTELLIGENCE_OUTPUT")))
	if outputDefault == "" {
		outputDefault = "memory"
	}

	target := flag.String("target", targetDefault, "exact case-sensitive chain target")
	network := flag.String("network", networkDefault, "exact network label")
	sinceRaw := flag.String("since", strings.TrimSpace(os.Getenv("KOSCHEI_CLICKHOUSE_INTELLIGENCE_SINCE")), "RFC3339 lower bound; defaults to 24h before until")
	untilRaw := flag.String("until", strings.TrimSpace(os.Getenv("KOSCHEI_CLICKHOUSE_INTELLIGENCE_UNTIL")), "RFC3339 upper bound; defaults to now")
	limit := flag.Uint64("limit", limitDefault, "maximum rows per ClickHouse memory source")
	output := flag.String("output", outputDefault, "output contract: memory or investigation")
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
	snapshot, err := client.ReadARVISMemory(ctx, clickhouse.ARVISMemoryReadRequest{
		Target: strings.TrimSpace(*target), Network: strings.TrimSpace(*network), Since: since, Until: until, Limit: *limit,
	})
	if err != nil {
		return err
	}

	var payload any = snapshot
	switch strings.ToLower(strings.TrimSpace(*output)) {
	case "memory":
	case "investigation":
		investigation, err := clickhouse.ProjectARVISMemory(snapshot, now)
		if err != nil {
			return fmt.Errorf("project intelligence investigation: %w", err)
		}
		payload = investigation
	default:
		return fmt.Errorf("unsupported output contract %q; expected memory or investigation", strings.TrimSpace(*output))
	}

	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(payload); err != nil {
		return fmt.Errorf("encode intelligence output: %w", err)
	}
	return nil
}
