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
	verdictMigrationPath       = "clickhouse/migrations/003_arvis_verdict_snapshots.sql"
	verdictMigrationDigestPath = verdictMigrationPath + ".sha256"
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

	if strings.TrimSpace(os.Getenv("KOSCHEI_CLICKHOUSE_VERDICT_SCHEMA_APPLY")) == "1" {
		migrationSQL, err := os.ReadFile(verdictMigrationPath)
		if err != nil {
			return fmt.Errorf("read trusted ClickHouse verdict migration %s: %w", verdictMigrationPath, err)
		}
		digestFile, err := os.ReadFile(verdictMigrationDigestPath)
		if err != nil {
			return fmt.Errorf("read trusted ClickHouse verdict migration checksum %s: %w", verdictMigrationDigestPath, err)
		}
		digestFields := strings.Fields(string(digestFile))
		if len(digestFields) != 2 || digestFields[1] != "003_arvis_verdict_snapshots.sql" {
			return fmt.Errorf("trusted ClickHouse verdict migration checksum manifest is malformed")
		}
		if err := koscheiclickhouse.VerifyTrustedMigrationSHA256(string(migrationSQL), digestFields[0]); err != nil {
			return err
		}
		if err := client.ApplyTrustedVerdictShadowMigration(ctx, string(migrationSQL)); err != nil {
			return err
		}
		log.Printf("ClickHouse additive verdict schema migration applied path=%s checksum_verified=true", verdictMigrationPath)
	}
	if err := client.VerifyVerdictShadowSchema(ctx); err != nil {
		return err
	}
	log.Printf("ClickHouse ARVIS verdict shadow schema verified database=%s", koscheiclickhouse.DefaultDatabase)
	return nil
}
