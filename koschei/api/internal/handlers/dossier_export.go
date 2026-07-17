package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
)

func (h *Handler) DossierExport(w http.ResponseWriter, r *http.Request) {
	mint := strings.TrimSpace(strings.TrimPrefix(r.URL.Path, "/api/v1/dossier/"))
	if mint == "" || strings.Contains(mint, "/") {
		writeAPIError(w, http.StatusBadRequest, APICodeInvalidInput, "mint is required")
		return
	}
	snapshot, err := h.loadDossierSnapshot(r.Context(), mint, strings.TrimSpace(r.URL.Query().Get("verdict_id")))
	if err != nil {
		switch {
		case errors.Is(err, errDossierSourceIncomplete), errors.Is(err, context.Canceled):
			writeJSON(w, http.StatusConflict, map[string]string{
				"error": "dossier_source_incomplete",
				"message": "An immutable signed scan snapshot is required; export never rescans or refreshes missing evidence.",
			})
		case errors.Is(err, errDossierSourceHash):
			writeJSON(w, http.StatusConflict, map[string]string{"error": "dossier_source_hash_mismatch"})
		default:
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "dossier_unavailable"})
		}
		return
	}
	bundle, canonical, err := assembleDossierBundle(snapshot)
	if err != nil {
		code := "dossier_assembly_failed"
		if errors.Is(err, errDossierReferenceMissing) { code = "dossier_evidence_reference_missing" }
		writeJSON(w, http.StatusUnprocessableEntity, map[string]string{"error": code, "message": err.Error()})
		return
	}
	if stored, ok := h.loadStoredDossierBytes(r.Context(), bundle.CaseRef); ok {
		writeDossierJSON(w, stored)
		return
	}
	if err := h.storeDossierBundle(r.Context(), snapshot, bundle, canonical, dossierRequester(r)); err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "dossier_store_failed"})
		return
	}
	stored, ok := h.loadStoredDossierBytes(r.Context(), bundle.CaseRef)
	if !ok {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "dossier_store_verification_failed"})
		return
	}
	writeDossierJSON(w, stored)
}

func assembleDossierBundle(snapshot dossierSnapshot) (dossierBundle, []byte, error) {
	if snapshot.Report == nil || strings.TrimSpace(snapshot.Mint) == "" || strings.TrimSpace(snapshot.VerdictSignature) == "" {
		return dossierBundle{}, nil, errDossierSourceIncomplete
	}
	rows := buildDossierSignalRows(snapshot.Report)
	for _, row := range rows {
		if (row.State == "verified" || row.State == "observed") && !dossierRefsPresent(row.Refs) {
			return dossierBundle{}, nil, fmt.Errorf("%w: %s", errDossierReferenceMissing, row.ID)
		}
	}
	caseRef := dossierCaseRef(snapshot.Mint, snapshot.VerdictSignature)
	body := dossierBody{
		DossierVersion: dossierVersion, CaseRef: caseRef, ProducedAt: snapshot.ProducedAt.UTC(),
		SourceSnapshotHash: snapshot.SourceHash,
		Token: map[string]any{
			"mint": snapshot.Mint, "network": snapshot.Network,
			"market_snapshot": snapshot.Report["market"],
			"launch_metadata": snapshot.Report["launch_forensics"],
			"source_context": snapshot.Report["source_context"],
		},
		Verdict: snapshot.Report["final_verdict"],
		VerdictCard: map[string]any{
			"mapper_id": "koschei-verdict-card", "mapper_version": dossierMapperVersion,
			"signal_rows": rows,
		},
		ThreatAnticipation: snapshot.Report["threat_anticipation"],
		EvidenceArms: dossierFirst(snapshot.Report["evidence_arms"], snapshot.Report["modules"], []any{}),
		TransactionEvidence: dossierFirst(snapshot.Report["transaction_evidence"], []any{}),
		EvidenceReferences: dossierFirst(snapshot.Report["evidence_references"], map[string]any{}),
		ActorDossier: snapshot.Report["actor_investigation"],
		HolderContext: snapshot.Report["holder_concentration_context"],
		TechnicalReport: snapshot.Report,
		Verification: map[string]any{
			"verifier_repo_url": dossierVerifierRepo,
			"verdict_signature": snapshot.VerdictSignature,
			"hash_algorithm": "SHA-256",
			"bundle_hash_scope": "UTF-8 JSON encoding of the dossier body with bundle_hash excluded; Go struct field order is fixed and map keys are lexicographically sorted by encoding/json.",
			"case_ref_rule": "KD1- + lower-case base32(no padding) of the first 20 SHA-256 bytes of mint + newline + verdict_signature.",
			"command": "node oss/verifier/typescript/verify-dossier.mjs ./dossier.json",
		},
		Limitations: append([]string{}, dossierLimitations...),
	}
	bodyBytes, err := json.Marshal(body)
	if err != nil { return dossierBundle{}, nil, err }
	bundle := dossierBundle{dossierBody: body, BundleHash: dossierSHA256(bodyBytes)}
	canonical, err := json.Marshal(bundle)
	if err != nil { return dossierBundle{}, nil, err }
	return bundle, canonical, nil
}

func writeDossierJSON(w http.ResponseWriter, canonical []byte) {
	var bundle dossierBundle
	if json.Unmarshal(canonical, &bundle) != nil || bundle.CaseRef == "" || bundle.BundleHash == "" {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "dossier_canonical_bundle_invalid"})
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=koschei-%s.json", bundle.CaseRef))
	w.Header().Set("ETag", `"`+bundle.BundleHash+`"`)
	w.Header().Set("X-Koschei-Case-Ref", bundle.CaseRef)
	_, _ = w.Write(canonical)
}

func (h *Handler) storeDossierBundle(ctx context.Context, snapshot dossierSnapshot, bundle dossierBundle, canonical []byte, requestedBy string) error {
	if h == nil || h.DB == nil { return errDossierSourceIncomplete }
	raw, err := json.Marshal(bundle)
	if err != nil { return err }
	_, err = h.DB.ExecContext(ctx, `
		INSERT INTO dossier_exports
		(case_ref,mint,verdict_id,verdict_signature,source_snapshot_id,bundle_hash,canonical_bundle,bundle_json,requested_by)
		VALUES ($1,$2,NULLIF($3,''),$4,$5::uuid,$6,$7,$8::jsonb,$9)
		ON CONFLICT (case_ref) DO NOTHING`,
		bundle.CaseRef, snapshot.Mint, snapshot.VerdictID, snapshot.VerdictSignature,
		snapshot.ID, bundle.BundleHash, canonical, string(raw), strings.TrimSpace(requestedBy),
	)
	return err
}

func (h *Handler) loadStoredDossierBytes(ctx context.Context, caseRef string) ([]byte, bool) {
	if h == nil || h.DB == nil { return nil, false }
	var raw []byte
	if h.DB.QueryRowContext(ctx, `SELECT canonical_bundle FROM dossier_exports WHERE case_ref=$1`, caseRef).Scan(&raw) != nil {
		return nil, false
	}
	return raw, len(raw) > 0
}

func dossierRequester(r *http.Request) string {
	if principal, ok := apiPrincipalFromContext(r.Context()); ok { return "api_key:" + strings.TrimSpace(principal.KeyID) }
	if user, ok := userFromContext(r.Context()); ok { return "user:" + strings.TrimSpace(user.Sub) }
	return "owner"
}
