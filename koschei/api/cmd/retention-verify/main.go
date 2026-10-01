package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"koschei/api/internal/services/retentionexport"

	_ "github.com/lib/pq"
)

func main() {
	var filePath string
	var expectedSHA256 string
	var restoreAcceptance bool
	flag.StringVar(&filePath, "file", "", "path to one exported retention NDJSON object")
	flag.StringVar(&expectedSHA256, "sha256", "", "expected SHA-256 of the exported object")
	flag.BoolVar(&restoreAcceptance, "restore-acceptance", false, "reconstruct the export into transaction-scoped PostgreSQL restore staging and verify parity")
	flag.Parse()

	filePath = strings.TrimSpace(filePath)
	if filePath == "" {
		exitError("retention export file is required")
	}
	databaseURL := strings.TrimSpace(os.Getenv("DATABASE_URL"))
	if databaseURL == "" {
		databaseURL = strings.TrimSpace(os.Getenv("APP_DATABASE_URL"))
	}
	if databaseURL == "" {
		exitError("DATABASE_URL or APP_DATABASE_URL is required")
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		exitError("read export object: " + err.Error())
	}
	db, err := sql.Open("postgres", databaseURL)
	if err != nil {
		exitError("open database: " + err.Error())
	}
	defer db.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		exitError("database ping: " + err.Error())
	}

	if restoreAcceptance {
		result, err := retentionexport.VerifyRestoreAcceptance(ctx, db, data, expectedSHA256)
		if err != nil {
			payload, _ := json.Marshal(map[string]any{"ok": false, "mode": "restore_acceptance", "result": result, "error": err.Error()})
			fmt.Fprintln(os.Stderr, string(payload))
			os.Exit(1)
		}
		payload, _ := json.Marshal(map[string]any{"ok": true, "mode": "restore_acceptance", "result": result})
		fmt.Println(string(payload))
		return
	}

	result, err := retentionexport.VerifyExportObject(ctx, db, data)
	if err != nil {
		payload, _ := json.Marshal(map[string]any{"ok": false, "mode": "verify", "result": result, "error": err.Error()})
		fmt.Fprintln(os.Stderr, string(payload))
		os.Exit(1)
	}
	payload, _ := json.Marshal(map[string]any{"ok": true, "mode": "verify", "result": result})
	fmt.Println(string(payload))
}

func exitError(message string) {
	payload, _ := json.Marshal(map[string]any{"ok": false, "error": message})
	fmt.Fprintln(os.Stderr, string(payload))
	os.Exit(2)
}
