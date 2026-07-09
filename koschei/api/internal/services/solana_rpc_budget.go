package services

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

var solanaRPCBudget = struct {
	sync.Mutex
	WindowStart time.Time
	Count       int
}{}

func reserveSolanaRPCBudget(ctx context.Context, method string) error {
	if !solanaRPCBudgetEnabled() {
		return nil
	}
	maxRequests := solanaRPCBudgetMaxRequests()
	if maxRequests <= 0 {
		return nil
	}
	window := solanaRPCBudgetWindow()
	now := time.Now()
	solanaRPCBudget.Lock()
	if solanaRPCBudget.WindowStart.IsZero() || now.Sub(solanaRPCBudget.WindowStart) >= window {
		solanaRPCBudget.WindowStart = now
		solanaRPCBudget.Count = 0
	}
	if solanaRPCBudget.Count < maxRequests {
		solanaRPCBudget.Count++
		solanaRPCBudget.Unlock()
		return nil
	}
	resetAt := solanaRPCBudget.WindowStart.Add(window)
	solanaRPCBudget.Unlock()
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	return fmt.Errorf("solana rpc budget exceeded for %s; next window at %s", strings.TrimSpace(method), resetAt.UTC().Format(time.RFC3339))
}

func resetSolanaRPCBudgetForTest() {
	solanaRPCBudget.Lock()
	solanaRPCBudget.WindowStart = time.Time{}
	solanaRPCBudget.Count = 0
	solanaRPCBudget.Unlock()
}

func SolanaRPCLimitSaverEnabled() bool {
	raw := strings.TrimSpace(os.Getenv("SOLANA_RPC_LIMIT_SAVER_ENABLED"))
	if raw != "" {
		enabled, err := strconv.ParseBool(raw)
		if err == nil {
			return enabled
		}
	}
	return strings.EqualFold(strings.TrimSpace(os.Getenv("APP_ENV")), "production")
}

func ForceBackgroundRadarEnabled() bool {
	return envBool("KOSCHEI_AUTO_RADAR_FORCE_BACKGROUND") || envBool("RADAR_STREAM_FORCE_ENABLED") || envBool("ARVIS_BACKGROUND_RPC_ENRICHMENT_ENABLED")
}

func solanaRPCBudgetEnabled() bool {
	raw := strings.TrimSpace(os.Getenv("SOLANA_RPC_BUDGET_ENABLED"))
	if raw != "" {
		enabled, err := strconv.ParseBool(raw)
		if err == nil {
			return enabled
		}
	}
	return SolanaRPCLimitSaverEnabled()
}

func solanaRPCBudgetMaxRequests() int {
	if raw := strings.TrimSpace(os.Getenv("SOLANA_RPC_BUDGET_MAX_REQUESTS")); raw != "" {
		if value, err := strconv.Atoi(raw); err == nil && value >= 0 && value <= 1000000 {
			return value
		}
	}
	if SolanaRPCLimitSaverEnabled() {
		return 220
	}
	return 0
}

func solanaRPCBudgetWindow() time.Duration {
	if raw := strings.TrimSpace(os.Getenv("SOLANA_RPC_BUDGET_WINDOW_SECONDS")); raw != "" {
		if value, err := strconv.Atoi(raw); err == nil && value >= 60 && value <= 86400 {
			return time.Duration(value) * time.Second
		}
	}
	return time.Hour
}

func solanaRPC429Cooldown() time.Duration {
	seconds := 60
	if SolanaRPCLimitSaverEnabled() {
		seconds = 300
	}
	if raw := strings.TrimSpace(os.Getenv("SOLANA_RPC_429_COOLDOWN_SECONDS")); raw != "" {
		if value, err := strconv.Atoi(raw); err == nil {
			seconds = value
		}
	}
	if seconds < 30 {
		seconds = 30
	}
	if seconds > 3600 {
		seconds = 3600
	}
	return time.Duration(seconds) * time.Second
}

func maxDuration(a, b time.Duration) time.Duration {
	if a > b {
		return a
	}
	return b
}
