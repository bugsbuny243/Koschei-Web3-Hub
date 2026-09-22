package services

import (
	"context"
	"database/sql"
	"log"
	"strings"
	"time"
)

func StartSecurityRadarGapHealerIfEnabled(ctx context.Context, db *sql.DB) func() {
	if db == nil || !securityRadarGapHealerEnabled() {
		return func() {}
	}
	if securityRadarStreamIngestMode() != securityRadarIngestModeJournal {
		log.Printf("security radar slot gap healer not started: journal ingest mode is required")
		return func() {}
	}
	if !securityRadarStreamEnabled() {
		return func() {}
	}
	rpcURL := resolveSecurityRadarRPCURL()
	if strings.TrimSpace(rpcURL) == "" {
		log.Printf("security radar slot gap healer not started: no Solana RPC URL could be resolved")
		return func() {}
	}
	childCtx, cancel := context.WithCancel(ctx)
	healer := newSecurityRadarGapHealer(NewSecurityRadarStore(db), rpcURL, "solana-mainnet")
	go func() {
		anchorCtx, anchorCancel := context.WithTimeout(childCtx, 30*time.Second)
		anchorErr := ensureSecurityRadarReplayAnchors(anchorCtx, healer)
		anchorCancel()
		if anchorErr != nil {
			if childCtx.Err() == nil {
				log.Printf("security radar slot gap healer not started: durable bootstrap anchor failed: %v", anchorErr)
			}
			return
		}
		if childCtx.Err() != nil {
			return
		}
		healer.Start(childCtx)
	}()
	return cancel
}
