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
	publicationMigrationPath       = "clickhouse/migrations/002_dossier_publication_ledger.sql"
	publicationMigrationDigestPath = publicationMigrationPath + ".sha256"
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

	if strings.TrimSpace(os.Getenv("KOSCHEI_CLICKHOUSE_PUBLICATION_SCHEMA_APPLY")) == "1" {
		migrationSQL, err := os.ReadFile(publicationMigrationPath)
		if err != nil {
			return fmt.Errorf("read trusted ClickHouse publication migration %s: %w", publicationMigrationPath, err)
		}
		digestFile, err := os.ReadFile(publicationMigrationDigestPath)
		if err != nil {
			return fmt.Errorf("read trusted ClickHouse publication migration checksum %s: %w", publicationMigrationDigestPath, err)
		}
		fields := strings.Fields(string(digestFile))
		if len(fields) != 2 || fields[1] != "002_dossier_publication_ledger.sql" {
			return fmt.Errorf("trusted ClickHouse publication migration checksum manifest is malformed")
		}
		if err := koscheiclickhouse.VerifyTrustedMigrationSHA256(string(migrationSQL), fields[0]); err != nil {
			return err
		}
		if err := client.ApplyTrustedPublicationMigration(ctx, string(migrationSQL)); err != nil {
			return err
		}
		log.Printf("ClickHouse additive publication migration applied path=%s checksum_verified=true", publicationMigrationPath)
	}

	if err := client.VerifyPublicationLedgerSchema(ctx); err != nil {
		return err
	}
	log.Printf("ClickHouse publication ledger schema verified database=%s", koscheiclickhouse.DefaultDatabase)
	return nil
}
