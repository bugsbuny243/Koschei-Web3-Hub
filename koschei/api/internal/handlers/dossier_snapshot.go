package handlers

import (
	"context"
	"encoding/json"
	"strings"
)

// persistDossierSourceSnapshot freezes the exact signed technical report. It is
// best-effort for the scan path; export never rescans when a snapshot is absent.
func (h *Handler) persistDossierSourceSnapshot(ctx context.Context, report map[string]any) error {
	if h == nil || h.DB == nil || report == nil { return nil }
	final := dossierMap(report["final_verdict"])
	signature := strings.TrimSpace(dossierString(final["signature"]))
	if signature == "" || !dossierBool(final["signed"]) { return nil }
	mint := strings.TrimSpace(dossierString(report["target"]))
	if mint == "" { return nil }
	network := firstNonEmptyString(dossierString(report["network"]), "solana-mainnet")
	ruleset := strings.TrimSpace(dossierString(final["ruleset_version"]))
	if ruleset == "" { return errDossierSourceIncomplete }
	producedAt := dossierParseTime(firstNonEmptyString(dossierString(final["generated_at"]), dossierString(report["generated_at"])))
	if producedAt.IsZero() { return errDossierSourceIncomplete }
	canonical, err := json.Marshal(report)
	if err != nil { return err }
	sourceHash := dossierSHA256(canonical)
	verdictID := strings.TrimSpace(dossierString(final["id"]))
	_, err = h.DB.ExecContext(ctx, `
		INSERT INTO dossier_source_snapshots
		(mint,network,verdict_id,verdict_signature,ruleset_version,produced_at,source_hash,source_payload)
		VALUES ($1,$2,NULLIF($3,''),$4,$5,$6,$7,$8::jsonb)
		ON CONFLICT (verdict_signature) DO NOTHING`,
		mint, network, verdictID, signature, ruleset, producedAt.UTC(), sourceHash, string(canonical),
	)
	return err
}

func (h *Handler) loadDossierSnapshot(ctx context.Context, mint, verdictID string) (dossierSnapshot, error) {
	if h == nil || h.DB == nil { return dossierSnapshot{}, errDossierSourceIncomplete }
	query := `
		SELECT id::text,mint,network,COALESCE(verdict_id,''),verdict_signature,
		       ruleset_version,produced_at,source_hash,source_payload
		FROM dossier_source_snapshots
		WHERE mint=$1`
	args := []any{strings.TrimSpace(mint)}
	if strings.TrimSpace(verdictID) != "" {
		query += ` AND (id::text=$2 OR verdict_id=$2 OR verdict_signature=$2)`
		args = append(args, strings.TrimSpace(verdictID))
	}
	query += ` ORDER BY produced_at DESC,id DESC LIMIT 1`
	var snapshot dossierSnapshot
	var raw []byte
	if err := h.DB.QueryRowContext(ctx, query, args...).Scan(
		&snapshot.ID, &snapshot.Mint, &snapshot.Network, &snapshot.VerdictID,
		&snapshot.VerdictSignature, &snapshot.RulesetVersion, &snapshot.ProducedAt,
		&snapshot.SourceHash, &raw,
	); err != nil {
		return dossierSnapshot{}, errDossierSourceIncomplete
	}
	if json.Unmarshal(raw, &snapshot.Report) != nil || snapshot.Report == nil {
		return dossierSnapshot{}, errDossierSourceIncomplete
	}
	canonical, err := json.Marshal(snapshot.Report)
	if err != nil || dossierSHA256(canonical) != snapshot.SourceHash {
		return dossierSnapshot{}, errDossierSourceHash
	}
	return snapshot, nil
}
