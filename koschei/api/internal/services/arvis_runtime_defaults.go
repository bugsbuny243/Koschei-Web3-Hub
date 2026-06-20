package services

import (
	"os"
	"strings"
)

const defaultSolanaMainnetRPC = "https://api.mainnet-beta.solana.com"

func init() {
	if strings.TrimSpace(os.Getenv("PUMP_FUN_PROGRAM_ID")) == "" {
		_ = os.Setenv("PUMP_FUN_PROGRAM_ID", defaultPumpProgramID)
	}
	if strings.TrimSpace(os.Getenv("PUMP_SWAP_PROGRAM_ID")) == "" {
		_ = os.Setenv("PUMP_SWAP_PROGRAM_ID", defaultPumpSwapProgramID)
	}
	if strings.TrimSpace(os.Getenv("SOLANA_RPC_URL")) == "" {
		_ = os.Setenv("SOLANA_RPC_URL", resolvedArvisRPCURLFromEnv())
	}
	if strings.TrimSpace(os.Getenv("ARVIS_HEARTBEAT_SECONDS")) == "" {
		if hasConfiguredArvisRPCProvider() {
			_ = os.Setenv("ARVIS_HEARTBEAT_SECONDS", "20")
		} else {
			_ = os.Setenv("ARVIS_HEARTBEAT_SECONDS", "60")
		}
	}
	if strings.TrimSpace(os.Getenv("ARVIS_STREAM_VERDICT_SECONDS")) == "" {
		if hasConfiguredArvisRPCProvider() {
			_ = os.Setenv("ARVIS_STREAM_VERDICT_SECONDS", "12")
		} else {
			_ = os.Setenv("ARVIS_STREAM_VERDICT_SECONDS", "30")
		}
	}
}

func resolvedArvisRPCURLFromEnv() string {
	for _, key := range []string{"SOLANA_RPC_URL", "ALCHEMY_SOLANA_RPC_URL", "HELIUS_SOLANA_RPC_URL", "QUICKNODE_SOLANA_RPC_URL"} {
		if value := strings.TrimSpace(os.Getenv(key)); value != "" {
			return value
		}
	}
	if key := strings.TrimSpace(os.Getenv("ALCHEMY_API_KEY")); key != "" {
		return "https://solana-mainnet.g.alchemy.com/v2/" + key
	}
	return defaultSolanaMainnetRPC
}

func hasConfiguredArvisRPCProvider() bool {
	for _, key := range []string{"SOLANA_RPC_URL", "ALCHEMY_SOLANA_RPC_URL", "HELIUS_SOLANA_RPC_URL", "QUICKNODE_SOLANA_RPC_URL", "ALCHEMY_API_KEY"} {
		value := strings.TrimSpace(os.Getenv(key))
		if value == "" {
			continue
		}
		if key == "SOLANA_RPC_URL" && strings.Contains(strings.ToLower(value), "api.mainnet-beta.solana.com") {
			continue
		}
		return true
	}
	return false
}
