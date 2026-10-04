package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"koschei/api/internal/clickhouse"
)

var migrations = []string{"005_global_radar_graph_records.sql", "006_global_radar_events.sql", "007_global_radar_ingest_checkpoints.sql", "008_global_radar_ingest_gaps.sql"}

func main() {
	if err := run(context.Background()); err != nil {
		log.Fatal(err)
	}
}

func run(parent context.Context) error {
	client, err := clickhouse.NewFromEnv()
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(parent, 2*time.Minute)
	defer cancel()
	if strings.TrimSpace(os.Getenv("KOSCHEI_CLICKHOUSE_SCHEMA_APPLY")) == "1" {
		// Validate every manifest before executing any reviewed additive DDL.
		bodies := make([]string, 0, len(migrations))
		for _, name := range migrations {
			payload, err := os.ReadFile("clickhouse/migrations/" + name)
			if err != nil {
				return err
			}
			manifest, err := os.ReadFile("clickhouse/migrations/" + name + ".sha256")
			if err != nil {
				return err
			}
			fields := strings.Fields(string(manifest))
			if len(fields) != 2 || fields[1] != name {
				return fmt.Errorf("invalid checksum manifest: %s", name)
			}
			if err := clickhouse.VerifyTrustedMigrationSHA256(string(payload), fields[0]); err != nil {
				return err
			}
			bodies = append(bodies, string(payload))
		}
		for i, body := range bodies {
			if err := client.ApplyTrustedMigration(ctx, body); err != nil {
				return err
			}
			log.Printf("Global Radar additive migration applied path=%s checksum_verified=true", migrations[i])
		}
	}
	for _, verify := range []func(context.Context) error{client.VerifyGlobalRadarGraphSchema, client.VerifyGlobalRadarEventSchema, client.VerifyGlobalRadarIngestCheckpointSchema, client.VerifyGlobalRadarIngestGapSchema} {
		if err := verify(ctx); err != nil {
			return err
		}
	}
	log.Printf("Global Radar graph, events, checkpoint and gap schemas verified")
	return nil
}
