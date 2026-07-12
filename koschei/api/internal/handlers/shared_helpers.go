package handlers

import (
	"crypto/rand"
	"database/sql"
	"fmt"
	"os"
	"strings"
	"time"
)

func solanaRPCURL(network string, apiKey string) string {
	if apiKey != "" {
		switch strings.ToLower(network) {
		case "solana-mainnet", "mainnet", "mainnet-beta", "solana-mainnet-beta":
			return "https://solana-mainnet.g.alchemy.com/v2/" + apiKey
		case "solana-devnet", "devnet":
			return "https://solana-devnet.g.alchemy.com/v2/" + apiKey
		case "solana-testnet", "testnet":
			return "https://solana-testnet.g.alchemy.com/v2/" + apiKey
		}
	}

	switch strings.ToLower(network) {
	case "solana-mainnet", "mainnet", "mainnet-beta", "solana-mainnet-beta":
		return "https://api.mainnet-beta.solana.com"
	case "solana-testnet", "testnet":
		return "https://api.testnet.solana.com"
	default:
		return "https://api.devnet.solana.com"
	}
}

func aiProviderConfigured() bool {
	return strings.TrimSpace(os.Getenv("OPENAI_API_KEY")) != "" ||
		strings.TrimSpace(os.Getenv("TOGETHER_API_KEY")) != ""
}

func newID() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func normalizePlanTier(planTier string) string {
	return normalizePackageID(planTier)
}

func normalizePackageID(packageID string) string {
	switch strings.ToLower(strings.TrimSpace(packageID)) {
	case "starter":
		return "starter"
	case "builder", "pro", "professional":
		return "professional"
	case "studio", "enterprise":
		return "enterprise"
	default:
		return ""
	}
}

func nullTimePtr(value sql.NullTime) *time.Time {
	if !value.Valid {
		return nil
	}
	return &value.Time
}
