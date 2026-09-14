package handlers

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"

	"koschei/api/internal/services"
)

// resolveCanonicalCreatorSourceContext closes the entry-point gap between a
// manually submitted mint and the actor investigation pipeline. Existing
// chain/source evidence always wins. Helius metadata and history are
// discovery-only: they may start the actor investigation as OBSERVED evidence,
// but cannot mark the creator relation VERIFIED without canonical RPC signer,
// mint-reference and launch-semantics verification.
func (h *Handler) resolveCanonicalCreatorSourceContext(ctx context.Context, target, network, mode string, source map[string]any) map[string]any {
	out := cloneCreatorSourceContext(source)
	if !unifiedLiveEvidenceAllowed(mode) {
		out["creator_resolution_status"] = "not_requested"
		return out
	}

	// Existing source context may already contain an externally discovered
	// creator and create signature. Do not return early: canonical RPC must get a
	// chance to upgrade that relation from OBSERVED to VERIFIED.
	if creator := strings.TrimSpace(creatorIntelCleanString(out["creator_wallet"])); creator != "" {
		signature := strings.TrimSpace(firstNonEmptyString(
			creatorIntelCleanString(out["creation_signature"]),
			creatorIntelCleanString(out["launch_signature"]),
			creatorIntelCleanString(out["first_mint_signature"]),
			creatorIntelCleanString(out["signature"]),
		))
		verification := h.verifyCanonicalCreatorRelation(ctx, target, network, creator, signature)
		out = applyCanonicalCreatorVerification(out, verification)
		if verification.Verified {
			return out
		}
		out["creator_resolution_status"] = firstNonEmptyString(creatorIntelCleanString(out["creator_resolution_status"]), "source_context_observed")
		return out
	}

	rpcURL := creatorMetadataRPCURL()
	metadata := services.FetchHeliusTokenMetadata(ctx, rpcURL, target)
	out["creator_resolution"] = metadata
	out["creator_resolution_provider"] = metadata.Provider
	out["creator_resolution_status"] = metadata.Status
	if !metadata.Available {
		status := strings.TrimSpace(metadata.Status)
		if status == "" {
			status = "unavailable"
		}
		appendCreatorResolutionLimitation(out, fmt.Sprintf("Creator resolution failed: %s. Helius API key could not be resolved from the configured RPC URL or Helius metadata was unavailable.", status))
		return out
	}
	out["available"] = true
	out["network"] = firstNonEmptyString(strings.TrimSpace(network), "solana-mainnet")
	out["token_name"] = firstNonEmptyString(creatorIntelCleanString(out["token_name"]), metadata.Name)
	out["token_symbol"] = firstNonEmptyString(creatorIntelCleanString(out["token_symbol"]), metadata.Symbol)
	out["token_2022_extensions"] = metadata.OnchainExtensions
	if strings.TrimSpace(metadata.Creator) == "" {
		// DAS and the bounded mint-history path can legitimately miss an older
		// creation transaction. Restore the historical target-mint Helius lookup
		// as a narrow fallback without enabling broad actor portfolio crawling.
		archival := services.FetchHeliusTargetCreatorArchival(ctx, rpcURL, target)
		out["creator_archival_resolution"] = archival
		if strings.TrimSpace(archival.Creator) == "" || strings.TrimSpace(archival.Signature) == "" {
			out["creator_resolution_status"] = "metadata_without_creator"
			for _, limitation := range archival.Limitations {
				appendCreatorResolutionLimitation(out, limitation)
			}
			return out
		}

		observedAt := archival.ObservedAt
		if observedAt.IsZero() {
			observedAt = time.Now().UTC()
		}
		out["source"] = "helius_target_mint_archival"
		out["source_address"] = strings.TrimSpace(archival.Creator)
		out["creator_wallet"] = strings.TrimSpace(archival.Creator)
		out["creator_label"] = "Helius target-mint archival creator discovery"
		out["creator_relation_verified"] = false
		out["creator_relation_observed"] = true
		out["creator_resolution_provider"] = archival.Provider
		out["creator_resolution_status"] = "observed_external_attribution"
		out["creator_scope"] = "Target-mint archival Helius discovery only; canonical RPC create-transaction signer, mint-reference and launch-semantics verification is required before VERIFIED status."
		out["signature"] = strings.TrimSpace(archival.Signature)
		out["creation_signature"] = strings.TrimSpace(archival.Signature)
		out["launch_signature"] = strings.TrimSpace(archival.Signature)
		out["first_mint_signature"] = strings.TrimSpace(archival.Signature)
		out["observed_at"] = observedAt.UTC().Format(time.RFC3339)
		for _, limitation := range archival.Limitations {
			appendCreatorResolutionLimitation(out, limitation)
		}

		verification := h.verifyCanonicalCreatorRelation(ctx, target, network, archival.Creator, archival.Signature)
		return applyCanonicalCreatorVerification(out, verification)
	}

	observedAt := metadata.ObservedAt
	if !metadata.CreatedAt.IsZero() {
		observedAt = metadata.CreatedAt
	}
	if observedAt.IsZero() {
		observedAt = time.Now().UTC()
	}
	out["source"] = "helius_das_and_rpc"
	out["source_address"] = strings.TrimSpace(metadata.Creator)
	out["creator_wallet"] = strings.TrimSpace(metadata.Creator)
	out["creator_label"] = "Helius creator discovery"
	out["creator_relation_verified"] = false
	out["creator_relation_observed"] = true
	out["creator_resolution_status"] = "observed_external_attribution"
	out["creator_scope"] = "Helius DAS and transaction-history discovery only; canonical RPC create-transaction signer verification is required before VERIFIED status."
	out["signature"] = strings.TrimSpace(metadata.CreateTransaction)
	out["creation_signature"] = strings.TrimSpace(metadata.CreateTransaction)
	out["launch_signature"] = strings.TrimSpace(metadata.CreateTransaction)
	out["observed_at"] = observedAt.UTC().Format(time.RFC3339)
	if metadata.CreatedTime > 0 {
		out["created_time"] = metadata.CreatedTime
	}
	if strings.TrimSpace(metadata.FirstMintTransaction) != "" {
		out["first_mint_signature"] = strings.TrimSpace(metadata.FirstMintTransaction)
	}

	verification := h.verifyCanonicalCreatorRelation(ctx, target, network, metadata.Creator, firstNonEmptyString(metadata.CreateTransaction, metadata.FirstMintTransaction))
	return applyCanonicalCreatorVerification(out, verification)
}

func creatorMetadataRPCURL() string {
	if rpcURL := strings.TrimSpace(os.Getenv("HELIUS_SOLANA_RPC_URL")); rpcURL != "" {
		return rpcURL
	}

	for _, envName := range []string{"SOLANA_RPC_URL", "ALCHEMY_SOLANA_RPC_URL", "QUICKNODE_SOLANA_RPC_URL"} {
		rpcURL := strings.TrimSpace(os.Getenv(envName))
		if rpcURL == "" {
			continue
		}
		parsed, err := url.Parse(rpcURL)
		if err != nil {
			continue
		}
		if strings.Contains(strings.ToLower(parsed.Hostname()), "helius") {
			return rpcURL
		}
	}

	return strings.TrimSpace(creatorIntelRPCURL())
}

func appendCreatorResolutionLimitation(out map[string]any, message string) {
	message = strings.TrimSpace(message)
	if message == "" {
		return
	}

	switch existing := out["limitations"].(type) {
	case []string:
		out["limitations"] = append(existing, message)
	case []any:
		out["limitations"] = append(existing, message)
	case string:
		existing = strings.TrimSpace(existing)
		if existing == "" {
			out["limitations"] = []string{message}
			return
		}
		out["limitations"] = []string{existing, message}
	case nil:
		out["limitations"] = []string{message}
	default:
		out["limitations"] = []string{message}
	}
}

func cloneCreatorSourceContext(source map[string]any) map[string]any {
	out := map[string]any{}
	for key, value := range source {
		out[key] = value
	}
	if _, ok := out["available"]; !ok {
		out["available"] = false
	}
	if _, ok := out["identity_claimed"]; !ok {
		out["identity_claimed"] = false
	}
	return out
}
