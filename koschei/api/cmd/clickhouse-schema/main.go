package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	koscheiclickhouse "koschei/api/internal/clickhouse"
)

const (
	migrationPath       = "clickhouse/migrations/001_security_radar_stream_events.sql"
	migrationDigestPath = migrationPath + ".sha256"
)

func main() {
	if err := run(context.Background()); err != nil {
		log.Fatal(err)
	}
}

func run(parent context.Context) error {
	client, err := koscheiclickhouse.NewFromEnv()
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(parent, 60*time.Second)
	defer cancel()

	if strings.TrimSpace(os.Getenv("KOSCHEI_CLICKHOUSE_SCHEMA_APPLY")) == "1" {
		migrationSQL, err := os.ReadFile(migrationPath)
		if err != nil {
			return fmt.Errorf("read trusted ClickHouse migration %s: %w", migrationPath, err)
		}
		digestFile, err := os.ReadFile(migrationDigestPath)
		if err != nil {
			return fmt.Errorf("read trusted ClickHouse migration checksum %s: %w", migrationDigestPath, err)
		}
		digestFields := strings.Fields(string(digestFile))
		if len(digestFields) != 2 || digestFields[1] != "001_security_radar_stream_events.sql" {
			return fmt.Errorf("trusted ClickHouse migration checksum manifest is malformed")
		}
		if err := koscheiclickhouse.VerifyTrustedMigrationSHA256(string(migrationSQL), digestFields[0]); err != nil {
			return err
		}
		if err := client.ApplyTrustedMigration(ctx, string(migrationSQL)); err != nil {
			return err
		}
		log.Printf("ClickHouse additive schema migration applied path=%s checksum_verified=true", migrationPath)
	}

	if err := client.VerifyStreamEventsSchema(ctx); err != nil {
		return err
	}
	log.Printf("ClickHouse stream schema verified database=%s", koscheiclickhouse.DefaultDatabase)
	return nil
}
