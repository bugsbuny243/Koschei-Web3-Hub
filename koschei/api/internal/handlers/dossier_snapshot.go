package handlers

import (
	"context"
	"encoding/json"
	"strings"
	"time"
)

// persistDossierSourceSnapshot freezes the exact signed technical report. It is
// best-effort for the scan path; export never rescans when a snapshot is absent.
func (h *Handler) persistDossierSourceSnapshot(ctx context.Context, report map[string]any) error {
	if h == nil || report == nil {
		return nil
	}
	if _, attached := report["defense_agent_runtime"]; !attached {
		target := strings.TrimSpace(dossierString(report["target"]))
		network := firstNonEmptyString(dossierString(report["network"]), "solana-mainnet")
		generatedAt := dossierParseTime(dossierString(report["generated_at"]))
		if generatedAt.IsZero() {
			generatedAt = time.Now().UTC()
		}
		h.attachDefenseAgentRuntime(ctx, report, target, network, generatedAt)
	}

	// Diagnostics mutate the exact canonical report before hashing. Wallet targets
	// never fabricate token-only ARVIS coverage; token targets keep the complete
	// token + actor reachability contract.
	if strings.EqualFold(strings.TrimSpace(dossierString(report["analysis_scope"])), "wallet_actor_investigation") {
		attachCanonicalWalletIntegrationCoverage(report)
	} else {
		attachCanonicalInvestigationDiagnostics(report)
	}

	if h.DB == nil {
		return nil
	}
	final := dossierMap(report["final_verdict"])
	signature := strings.TrimSpace(dossierString(final["signature"]))
	if signature == "" || !dossierBool(final["signed"]) {
		return nil
	}
	target := strings.TrimSpace(dossierString(report["target"]))
	if target == "" {
		return nil
	}
	network := firstNonEmptyString(dossierString(report["network"]), "solana-mainnet")
	ruleset := strings.TrimSpace(dossierString(final["ruleset_version"]))
	if ruleset == "" {
		return errDossierSourceIncomplete
	}
	producedAt := dossierParseTime(firstNonEmptyString(dossierString(final["generated_at"]), dossierString(report["generated_at"])))
	if producedAt.IsZero() {
		return errDossierSourceIncomplete
	}
	canonical, err := json.Marshal(report)
	if err != nil {
		return err
	}
	sourceHash := dossierSHA256(canonical)
	verdictID := strings.TrimSpace(dossierString(final["id"]))
	_, err = h.DB.ExecContext(ctx, `
		INSERT INTO dossier_source_snapshots
		(mint,network,verdict_id,verdict_signature,ruleset_version,produced_at,source_hash,canonical_source,source_payload)
		VALUES ($1,$2,NULLIF($3,''),$4,$5,$6,$7,$8,$9::jsonb)
		ON CONFLICT (verdict_signature) DO NOTHING`,
		target, network, verdictID, signature, ruleset, producedAt.UTC(), sourceHash, canonical, string(canonical),
	)
	return err
}

func (h *Handler) loadDossierSnapshot(ctx context.Context, mint, verdictID string) (dossierSnapshot, error) {
	if h == nil || h.DB == nil {
		return dossierSnapshot{}, errDossierSourceIncomplete
	}
	query := `
		SELECT id::text,mint,network,COALESCE(verdict_id,''),verdict_signature,
		       ruleset_version,produced_at,source_hash,canonical_source
		FROM dossier_source_snapshots
		WHERE mint=$1`
	args := []any{strings.TrimSpace(mint)}
	if strings.TrimSpace(verdictID) != "" {
		query += ` AND (id::text=$2 OR verdict_id=$2 OR verdict_signature=$2)`
		args = append(args, strings.TrimSpace(verdictID))
	}
	query += ` ORDER BY produced_at DESC,id DESC LIMIT 1`
	var snapshot dossierSnapshot
	var canonical []byte
	if err := h.DB.QueryRowContext(ctx, query, args...).Scan(
		&snapshot.ID, &snapshot.Mint, &snapshot.Network, &snapshot.VerdictID,
		&snapshot.VerdictSignature, &snapshot.RulesetVersion, &snapshot.ProducedAt,
		&snapshot.SourceHash, &canonical,
	); err != nil {
		return dossierSnapshot{}, errDossierSourceIncomplete
	}
	if dossierSHA256(canonical) != snapshot.SourceHash {
		return dossierSnapshot{}, errDossierSourceHash
	}
	if json.Unmarshal(canonical, &snapshot.Report) != nil || snapshot.Report == nil {
		return dossierSnapshot{}, errDossierSourceIncomplete
	}
	return snapshot, nil
}
