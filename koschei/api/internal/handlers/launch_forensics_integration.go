package handlers

import (
	"context"
	"database/sql"
	"os"
	"strconv"
	"strings"
	"time"

	"koschei/api/internal/services"
)

func (h *Handler) analyzeLaunchForensics(parent context.Context, target string, roles services.HolderRoleAnalysis, cluster services.HolderClusterAnalysis, source map[string]any) services.LaunchForensicsAnalysis {
	rpcURL := strings.TrimSpace(firstNonEmptyString(os.Getenv("SOLANA_RPC_URL"), os.Getenv("ALCHEMY_SOLANA_RPC_URL"), os.Getenv("HELIUS_SOLANA_RPC_URL")))
	creator := strings.TrimSpace(creatorIntelCleanString(source["creator_wallet"]))
	launchBlockTime := int64(0)
	if parsed, err := time.Parse(time.RFC3339, strings.TrimSpace(cluster.LaunchEstimateAt)); err == nil {
		launchBlockTime = parsed.Unix()
	}
	if launchBlockTime == 0 {
		if eventType := strings.ToLower(strings.TrimSpace(creatorIntelCleanString(source["event_type"]))); strings.Contains(eventType, "new_token") || strings.Contains(eventType, "launch") {
			if parsed, err := time.Parse(time.RFC3339, strings.TrimSpace(creatorIntelCleanString(source["observed_at"]))); err == nil {
				launchBlockTime = parsed.Unix()
			}
		}
	}
	timeout := 120 * time.Second
	if raw := strings.TrimSpace(os.Getenv("ARVIS_FORENSICS_TIMEOUT_SECONDS")); raw != "" {
		if seconds, err := strconv.Atoi(raw); err == nil && seconds >= 30 && seconds <= 300 {
			timeout = time.Duration(seconds) * time.Second
		}
	}
	ctx, cancel := context.WithTimeout(parent, timeout)
	defer cancel()
	return services.AnalyzeLaunchForensics(ctx, h.launchForensicsDB(), rpcURL, target, creator, roles, launchBlockTime, cluster.LaunchEstimateSlot)
}

func (h *Handler) launchForensicsDB() *sql.DB {
	if h == nil {
		return nil
	}
	if h.DBRead != nil {
		return h.DBRead
	}
	return h.DB
}
