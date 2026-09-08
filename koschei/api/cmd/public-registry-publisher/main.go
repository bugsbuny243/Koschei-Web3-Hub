package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"koschei/api/internal/db"
	"koschei/api/internal/handlers"
)

const (
	defaultPublishTimeoutSeconds = 120
	minPublishTimeoutSeconds     = 10
	maxPublishTimeoutSeconds     = 600
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	databaseURL := firstNonEmptyEnv("DATABASE_READ_URL", "DATABASE_URL")
	if databaseURL == "" {
		return fmt.Errorf("DATABASE_READ_URL or DATABASE_URL is required")
	}
	if strings.TrimSpace(os.Getenv("GOOGLE_DRIVE_ARCHIVE_FOLDER_ID")) == "" {
		return fmt.Errorf("GOOGLE_DRIVE_ARCHIVE_FOLDER_ID is required")
	}
	if strings.TrimSpace(os.Getenv("GOOGLE_DRIVE_SERVICE_ACCOUNT_JSON")) == "" {
		return fmt.Errorf("GOOGLE_DRIVE_SERVICE_ACCOUNT_JSON is required")
	}
	timeout, err := publishTimeout()
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	database, err := db.ConnectReplica(databaseURL)
	if err != nil {
		return fmt.Errorf("connect public registry source database: %w", err)
	}
	defer database.Close()

	h := &handlers.Handler{DB: database, DBRead: database}
	receipt, err := h.PublishPublicDossierRegistrySnapshot(ctx)
	if err != nil {
		return err
	}
	log.Printf(
		"public registry snapshot published: object=%s sha256=%s generated_at=%s total_publications=%d published_cases=%d registry_status=%s publication_ledger_status=%s readback_verified=%t",
		receipt.ObjectName,
		receipt.ObjectSHA256,
		receipt.GeneratedAt.UTC().Format(time.RFC3339Nano),
		receipt.TotalPublications,
		receipt.PublishedCases,
		receipt.RegistryStatus,
		receipt.PublicationLedgerStatus,
		receipt.ReadbackVerified,
	)
	return nil
}

func firstNonEmptyEnv(names ...string) string {
	for _, name := range names {
		if value := strings.TrimSpace(os.Getenv(name)); value != "" {
			return value
		}
	}
	return ""
}

func publishTimeout() (time.Duration, error) {
	raw := strings.TrimSpace(os.Getenv("KOSCHEI_PUBLIC_REGISTRY_PUBLISH_TIMEOUT_SECONDS"))
	if raw == "" {
		return defaultPublishTimeoutSeconds * time.Second, nil
	}
	seconds, err := strconv.Atoi(raw)
	if err != nil || seconds < minPublishTimeoutSeconds || seconds > maxPublishTimeoutSeconds {
		return 0, fmt.Errorf("KOSCHEI_PUBLIC_REGISTRY_PUBLISH_TIMEOUT_SECONDS must be between %d and %d", minPublishTimeoutSeconds, maxPublishTimeoutSeconds)
	}
	return time.Duration(seconds) * time.Second, nil
}
