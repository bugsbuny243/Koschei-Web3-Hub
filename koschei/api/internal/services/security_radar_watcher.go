package services

import (
	"context"
	"database/sql"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"koschei/api/internal/web3"
)

func SecurityRadarAutoEnabled() bool {
	value := strings.TrimSpace(os.Getenv("KOSCHEI_AUTO_RADAR_ENABLED"))
	return strings.EqualFold(value, "1") || strings.EqualFold(value, "true")
}

func StartSecurityRadarWatcher(ctx context.Context, db *sql.DB, _ *web3.SolanaRPC) func() {
	stopHeartbeat := StartArvisRadarHeartbeat(ctx, db)
	if db == nil || !SecurityRadarAutoEnabled() {
		return stopHeartbeat
	}
	rpcURL := firstSecurityRadarEnv("SOLANA_RPC_URL", "ALCHEMY_SOLANA_RPC_URL")
	if rpcURL == "" {
		log.Printf("security radar worker not started: SOLANA_RPC_URL is empty")
		return stopHeartbeat
	}
	pollEvery := 10 * time.Second
	if raw := strings.TrimSpace(os.Getenv("KOSCHEI_RADAR_POLL_SECONDS")); raw != "" {
		if seconds, err := strconv.Atoi(raw); err == nil && seconds > 0 {
			pollEvery = time.Duration(seconds) * time.Second
		}
	}
	worker := NewSecurityRadarWorker(NewSecurityRadarStore(db), rpcURL, true, pollEvery)
	ctx, cancel := context.WithCancel(ctx)
	go worker.Start(ctx)
	return func() {
		cancel()
		stopHeartbeat()
	}
}

func firstSecurityRadarEnv(keys ...string) string {
	for _, key := range keys {
		if value := strings.TrimSpace(os.Getenv(key)); value != "" {
			return value
		}
	}
	return ""
}
