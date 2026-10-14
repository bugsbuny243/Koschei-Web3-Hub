package handlers

import (
	"context"
	"net/http"
	"os"
	"strings"
	"time"
)

// collectUnifiedMintOwnerEvidence reads the mint owner directly from RPC.
// Unknown owners and failed calls never imply classic SPL Token.
func (h *Handler) collectUnifiedMintOwnerEvidence(parent context.Context, target, network string) map[string]any {
	unverified := map[string]any{"mint_owner_program_evidence_status": "unverified"}
	if h == nil || strings.TrimSpace(target) == "" || !strings.HasPrefix(strings.ToLower(network), "solana") {
		return unverified
	}
	ctx, cancel := context.WithTimeout(parent, 6500*time.Millisecond)
	defer cancel()
	var account struct {
		Value *struct {
			Owner string `json:"owner"`
			Data  struct {
				Parsed struct {
					Type string `json:"type"`
				} `json:"parsed"`
			} `json:"data"`
		} `json:"value"`
	}
	rpcURL := solanaRPCURL(network, os.Getenv("ALCHEMY_API_KEY"))
	if err := h.callSolanaRPC(ctx, &http.Client{Timeout: 7 * time.Second}, rpcURL, network, "getAccountInfo", []interface{}{target, map[string]string{"encoding": "jsonParsed"}}, &account); err != nil || account.Value == nil {
		return unverified
	}
	if !strings.EqualFold(strings.TrimSpace(account.Value.Data.Parsed.Type), "mint") {
		return unverified
	}
	return mintOwnerProgramEvidence(account.Value.Owner)
}
