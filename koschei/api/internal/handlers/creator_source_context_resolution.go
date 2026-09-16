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

	metadata := services.FetchHeliusTokenMetadata(ctx, creatorMetadataRPCURL(), target)
	out["creator_resolution"] = metadata
	out["creator_resolution_provider"] = metadata.Provider
	out["creator_resolution_status"] = metadata.Status
	if !metadata.Available {
		out["limitations"] = appendCreatorResolutionLimitation(out["limitations"], metadata.Status, metadata.Limitations)
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

// creatorMetadataRPCURL deliberately prefers an RPC URL that can resolve a
// Helius enhanced API key without changing creatorIntelRPCURL, which remains
// the canonical RPC preference for the rest of creator intelligence.
func creatorMetadataRPCURL() string {
	if heliusRPC := strings.TrimSpace(os.Getenv("HELIUS_SOLANA_RPC_URL")); heliusRPC != "" {
		return heliusRPC
	}
	for _, envName := range []string{"SOLANA_RPC_URL", "ALCHEMY_SOLANA_RPC_URL", "QUICKNODE_SOLANA_RPC_URL"} {
		candidate := strings.TrimSpace(os.Getenv(envName))
		if candidate != "" && creatorMetadataHeliusHostname(candidate) {
			return candidate
		}
	}
	return strings.TrimSpace(creatorIntelRPCURL())
}

func creatorMetadataHeliusHostname(rawURL string) bool {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return false
	}
	return strings.Contains(strings.ToLower(parsed.Hostname()), "helius")
}

func appendCreatorResolutionLimitation(raw any, status string, metadataLimitations []string) []string {
	limitations := make([]string, 0, len(metadataLimitations)+2)
	switch values := raw.(type) {
	case []string:
		limitations = append(limitations, values...)
	case []any:
		for _, value := range values {
			if text := strings.TrimSpace(creatorIntelCleanString(value)); text != "" {
				limitations = append(limitations, text)
			}
		}
	case string:
		if text := strings.TrimSpace(values); text != "" {
			limitations = append(limitations, text)
		}
	}

	status = firstNonEmptyString(strings.TrimSpace(status), "unknown")
	message := fmt.Sprintf("Creator resolution failed: %s.", status)
	if status == "not_configured" {
		message += " Helius API key could not be resolved from the configured RPC URL."
	} else if len(metadataLimitations) > 0 {
		message += " " + strings.TrimSpace(metadataLimitations[0])
	}
	limitations = append(limitations, message)
	return limitations
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
