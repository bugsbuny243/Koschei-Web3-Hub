package services

import (
	"context"
	"database/sql"
	"log"
	"os"
	"strconv"
	"strings"
	"time"
)

func StartArvisRadarHeartbeat(ctx context.Context, db *sql.DB) func() {
	if db == nil || envBool("ARVIS_HEARTBEAT_DISABLED") {
		return func() {}
	}
	rpcURL := firstSecurityRadarEnv("SOLANA_RPC_URL", "ALCHEMY_SOLANA_RPC_URL", "HELIUS_SOLANA_RPC_URL", "QUICKNODE_SOLANA_RPC_URL")
	if rpcURL == "" {
		if key := strings.TrimSpace(os.Getenv("ALCHEMY_API_KEY")); key != "" {
			rpcURL = "https://solana-mainnet.g.alchemy.com/v2/" + key
		}
	}
	if rpcURL == "" {
		rpcURL = "https://api.mainnet-beta.solana.com"
	}
	ctx, cancel := context.WithCancel(ctx)
	go arvisRadarHeartbeatLoop(ctx, NewSecurityRadarStore(db), rpcURL)
	return cancel
}

func arvisRadarHeartbeatLoop(ctx context.Context, store *SecurityRadarStore, rpcURL string) {
	pollEvery := 20 * time.Second
	if raw := strings.TrimSpace(os.Getenv("ARVIS_HEARTBEAT_SECONDS")); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil && n >= 5 && n <= 300 {
			pollEvery = time.Duration(n) * time.Second
		}
	}
	log.Printf("arvis radar heartbeat started interval=%s", pollEvery)
	arvisRadarHeartbeatOnce(ctx, store, rpcURL)
	ticker := time.NewTicker(pollEvery)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			log.Printf("arvis radar heartbeat stopped")
			return
		case <-ticker.C:
			arvisRadarHeartbeatOnce(ctx, store, rpcURL)
		}
	}
}

func arvisRadarHeartbeatOnce(ctx context.Context, store *SecurityRadarStore, rpcURL string) {
	if store == nil || store.DB == nil || strings.TrimSpace(rpcURL) == "" {
		return
	}
	address := firstRadarValue(os.Getenv("RAYDIUM_PROGRAM_ID"), "675kPX9MHTjS2zt1qfr1NYhd1B9M9QGK6cEcDDCo2t9")
	pollCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	signatures, err := SolanaGetSignaturesForAddress(pollCtx, rpcURL, address, 3)
	cancel()
	if err != nil {
		log.Printf("arvis radar heartbeat poll failed: %v", err)
		return
	}
	for i := len(signatures) - 1; i >= 0; i-- {
		info := signatures[i]
		sig := strings.TrimSpace(info.Signature)
		if sig == "" || arvisHeartbeatEventExists(ctx, store.DB, sig) {
			continue
		}

		target := ""
		targetType := "program"
		evidenceQuality := "live_program_signature"
		decoded := map[string]any{"source": "raydium_program", "rpc_method": "getSignaturesForAddress"}
		txCtx, txCancel := context.WithTimeout(ctx, 7*time.Second)
		tx, txErr := SolanaGetTransactionJSONParsed(txCtx, rpcURL, sig)
		txCancel()
		if txErr == nil {
			mints := extractMintsFromTransactionMap(map[string]any(tx))
			selectedMint := selectArvisTargetMint(mints)
			decoded["enriched_mints"] = mints
			decoded["selected_project_mint"] = selectedMint
			decoded["base_asset_mints_filtered"] = selectedMint != "" && len(mints) > 1
			decoded["transaction_parsed"] = true
			if selectedMint != "" {
				target = selectedMint
				targetType = "token"
				evidenceQuality = "transaction_enriched_mint"
				decoded["enriched_mint"] = target
			}
		} else {
			decoded["transaction_parsed"] = false
			decoded["enrichment_error"] = compactRadarError("getTransaction", txErr)
		}

		_, err := store.InsertStreamEvent(ctx, SecurityRadarStreamEventRecord{
			Provider:        "solana_rpc",
			StreamMode:      "heartbeat_poll",
			Network:         "solana-mainnet",
			ModuleID:        ModuleRaydiumPoolGuardian,
			EventType:       "raydium_program_signature",
			Target:          target,
			TargetType:      targetType,
			Signature:       sig,
			Slot:            info.Slot,
			ProgramID:       address,
			EvidenceQuality: evidenceQuality,
			Decoded:         decoded,
			RawEvent:        map[string]any{"signature": sig, "slot": info.Slot, "target": target},
		})
		if err != nil {
			log.Printf("arvis radar heartbeat insert failed: %v", err)
		}
	}
}

func arvisHeartbeatEventExists(ctx context.Context, db *sql.DB, signature string) bool {
	if db == nil || strings.TrimSpace(signature) == "" {
		return false
	}
	var exists bool
	err := db.QueryRowContext(ctx, `
		SELECT EXISTS(
			SELECT 1 FROM security_radar_stream_events
			WHERE signature=$1 AND stream_mode='heartbeat_poll'
		)
	`, signature).Scan(&exists)
	return err == nil && exists
}
