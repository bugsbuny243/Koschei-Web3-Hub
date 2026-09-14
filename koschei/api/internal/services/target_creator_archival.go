package services

import (
	"context"
	"strings"
	"time"
)

// TargetCreatorArchivalObservation is discovery-only evidence for one target
// mint. It never upgrades creator attribution to VERIFIED; callers must re-read
// the referenced transaction from canonical RPC and verify signer, mint
// reference and launch semantics before using it as verified evidence.
type TargetCreatorArchivalObservation struct {
	Configured bool      `json:"configured"`
	Available  bool      `json:"available"`
	Status     string    `json:"status"`
	Provider   string    `json:"provider"`
	Mint       string    `json:"mint"`
	Creator    string    `json:"creator,omitempty"`
	Signature  string    `json:"signature,omitempty"`
	Slot       int64     `json:"slot,omitempty"`
	ObservedAt time.Time `json:"observed_at,omitempty"`
	Limitations []string `json:"limitations"`
}

// FetchHeliusTargetCreatorArchival performs a single-target archival creator
// lookup. This is intentionally separate from broad actor-wallet created-mint
// discovery so restoring creator resolution does not silently expand portfolio
// crawling or change its runtime policy.
func FetchHeliusTargetCreatorArchival(ctx context.Context, rpcURL, mint string) TargetCreatorArchivalObservation {
	mint = strings.TrimSpace(mint)
	out := TargetCreatorArchivalObservation{
		Status:      "not_configured",
		Provider:    "helius_target_mint_archival",
		Mint:        mint,
		Limitations: []string{},
	}
	if mint == "" {
		out.Status = "mint_required"
		out.Limitations = append(out.Limitations, "A token mint is required for target creator archival discovery.")
		return out
	}

	apiKey := heliusEnhancedAPIKey(rpcURL)
	if apiKey == "" {
		out.Limitations = append(out.Limitations, "No Helius API key resolved; target creator archival discovery was skipped.")
		return out
	}
	out.Configured = true
	endpoint := heliusRPCProviderURL(rpcURL, apiKey)
	observation, err := fetchHeliusMintCreationObservation(ctx, endpoint, mint)
	if err != nil {
		out.Status = "collection_failed"
		out.Limitations = append(out.Limitations, "Helius target-mint archival history failed: "+compactClusterError(err))
		return out
	}
	out.Available = true
	if strings.TrimSpace(observation.Signature) == "" || strings.TrimSpace(observation.Creator) == "" {
		out.Status = "creator_not_observed"
		out.Limitations = append(out.Limitations, "No creator signer and exact creation transaction were observed in the target-mint archival response.")
		return out
	}

	out.Status = "observed_external_attribution"
	out.Creator = strings.TrimSpace(observation.Creator)
	out.Signature = strings.TrimSpace(observation.Signature)
	out.Slot = observation.Slot
	if observation.BlockTime > 0 {
		out.ObservedAt = time.Unix(observation.BlockTime, 0).UTC()
	}
	out.Limitations = append(out.Limitations, "Archival creator attribution is OBSERVED discovery evidence only; canonical RPC verification is required before VERIFIED status.")
	return out
}
