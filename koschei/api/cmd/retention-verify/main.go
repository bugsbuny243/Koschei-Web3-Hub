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
	flag.StringVar(&filePath, "file", "", "path to one exported retention NDJSON object")
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

	result, err := retentionexport.VerifyExportObject(ctx, db, data)
	if err != nil {
		payload, _ := json.Marshal(map[string]any{"ok": false, "result": result, "error": err.Error()})
		fmt.Fprintln(os.Stderr, string(payload))
		os.Exit(1)
	}
	payload, _ := json.Marshal(map[string]any{"ok": true, "result": result})
	fmt.Println(string(payload))
}

func exitError(message string) {
	payload, _ := json.Marshal(map[string]any{"ok": false, "error": message})
	fmt.Fprintln(os.Stderr, string(payload))
	os.Exit(2)
}
