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
	stopStreamVerdicts := StartArvisStreamVerdictWorker(ctx, db)
	stopStreamRecovery := StartArvisStreamRecovery(ctx, db)
	stopAll := func() {
		stopStreamRecovery()
		stopStreamVerdicts()
		stopHeartbeat()
	}
	if db == nil || !SecurityRadarAutoEnabled() {
		return stopAll
	}
	rpcURL := firstSecurityRadarEnv("SOLANA_RPC_URL", "ALCHEMY_SOLANA_RPC_URL")
	if rpcURL == "" {
		log.Printf("security radar polling worker not started: SOLANA_RPC_URL is empty")
		return stopAll
	}
	pollEvery := 60 * time.Second
	if raw := strings.TrimSpace(os.Getenv("KOSCHEI_RADAR_POLL_SECONDS")); raw != "" {
		if seconds, err := strconv.Atoi(raw); err == nil && seconds >= 30 && seconds <= 600 {
			pollEvery = time.Duration(seconds) * time.Second
		}
	}
	worker := NewSecurityRadarWorker(NewSecurityRadarStore(db), rpcURL, true, pollEvery)
	pollCtx, cancelPolling := context.WithCancel(ctx)
	go worker.Start(pollCtx)
	return func() {
		cancelPolling()
		stopAll()
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
