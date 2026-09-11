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
	readIndexMigrationPath       = "clickhouse/migrations/004_arvis_memory_read_indexes.sql"
	readIndexMigrationDigestPath = readIndexMigrationPath + ".sha256"
)

func main() {
	if err := run(context.Background()); err != nil {
		log.Fatal(err)
	}
}

func run(parent context.Context) error {
	if strings.TrimSpace(os.Getenv("KOSCHEI_CLICKHOUSE_ARVIS_READ_INDEXES_APPLY")) != "1" {
		return fmt.Errorf("refusing ClickHouse ARVIS read-index apply without KOSCHEI_CLICKHOUSE_ARVIS_READ_INDEXES_APPLY=1")
	}
	client, err := koscheiclickhouse.NewFromEnv()
	if err != nil {
		return err
	}
	migrationSQL, err := os.ReadFile(readIndexMigrationPath)
	if err != nil {
		return fmt.Errorf("read trusted ClickHouse ARVIS read-index migration %s: %w", readIndexMigrationPath, err)
	}
	digestFile, err := os.ReadFile(readIndexMigrationDigestPath)
	if err != nil {
		return fmt.Errorf("read trusted ClickHouse ARVIS read-index migration checksum %s: %w", readIndexMigrationDigestPath, err)
	}
	fields := strings.Fields(string(digestFile))
	if len(fields) != 2 || fields[1] != "004_arvis_memory_read_indexes.sql" {
		return fmt.Errorf("trusted ClickHouse ARVIS read-index migration checksum manifest is malformed")
	}
	if err := koscheiclickhouse.VerifyTrustedMigrationSHA256(string(migrationSQL), fields[0]); err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(parent, 60*time.Second)
	defer cancel()
	if err := client.ApplyTrustedVerdictShadowMigration(ctx, string(migrationSQL)); err != nil {
		return fmt.Errorf("apply ClickHouse ARVIS read indexes: %w", err)
	}
	log.Printf("ClickHouse ARVIS read-index migration applied path=%s checksum_verified=true", readIndexMigrationPath)
	return nil
}
