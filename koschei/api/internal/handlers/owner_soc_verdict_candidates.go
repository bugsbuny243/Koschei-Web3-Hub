package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"koschei/api/internal/services"
)

const ownerSOCVerdictCandidateLimit = 20
const ownerSOCVerdictCandidateBoundary = "read_only_trusted_historical_verdicts; candidate_links_are_not_soc_incidents; no_auto_severity_or_enforcement; owner_action_required"

type ownerSOCVerdictCandidate struct {
	Network     string    `json:"network"`
	Target      string    `json:"target"`
	Grade       string    `json:"grade"`
	Verdict     string    `json:"verdict"`
	EvidenceRef string    `json:"evidence_ref"`
	RecordedAt  time.Time `json:"recorded_at"`
	Linked      bool      `json:"linked_to_soc"`
}

// OwnerSOCVerdictCandidates lists only independently verified immutable ARVIS
// receipts under the existing ownerOnly authorization boundary. It cannot
// create incidents or upgrade a signed verdict into an automatic response.
func (h *Handler) OwnerSOCVerdictCandidates(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	if h == nil || h.DB == nil {
		writeAPIError(w, http.StatusServiceUnavailable, APICodeServiceUnavailable, "SOC signed candidate store unavailable")
		return
	}
	db := h.DBRead
	if db == nil {
		db = h.DB
	}
	registry, err := services.SOCUnifiedVerdictTrustRegistry()
	if err != nil {
		writeAPIError(w, http.StatusServiceUnavailable, APICodeServiceUnavailable, "SOC independent signed verdict trust unavailable")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	rows, err := db.QueryContext(ctx, `
		SELECT receipt.id::text,receipt.network,receipt.target_id,
		       receipt.signature,receipt.signature_algorithm,receipt.key_id,
		       receipt.payload_hash,receipt.recorded_at,
		       CASE WHEN octet_length(receipt.signed_payload) BETWEEN 1 AND 65536
		            THEN receipt.signed_payload ELSE NULL END,
		       EXISTS (
		         SELECT 1 FROM security_incident_cases incident
		         WHERE incident.network=receipt.network AND incident.target=receipt.target_id
		           AND jsonb_exists(incident.evidence_refs,
		             'unified-verdict-ledger:' || receipt.id::text || ':' || receipt.signature)
		       )
		FROM security_unified_verdict_receipts receipt
		WHERE receipt.target_kind='token'
		ORDER BY receipt.recorded_at DESC,receipt.id DESC
		LIMIT 20
	`)
	if err != nil {
		writeAPIError(w, http.StatusServiceUnavailable, APICodeServiceUnavailable, "SOC signed verdict candidate read unavailable")
		return
	}
	defer rows.Close()
	items := make([]ownerSOCVerdictCandidate, 0, ownerSOCVerdictCandidateLimit)
	for rows.Next() {
		var id, network, target, signature, algorithm, keyID, hash string
		var recordedAt time.Time
		var payload []byte
		var linked bool
		if err := rows.Scan(&id, &network, &target, &signature, &algorithm, &keyID,
			&hash, &recordedAt, &payload, &linked); err != nil {
			writeAPIError(w, http.StatusServiceUnavailable, APICodeServiceUnavailable, "SOC signed receipt scan unavailable")
			return
		}
		if algorithm != services.UnifiedVerdictSignatureAlgorithmV1 || len(payload) == 0 {
			writeAPIError(w, http.StatusServiceUnavailable, APICodeServiceUnavailable, "SOC signed receipt verification failed")
			return
		}
		identity, err := services.VerifyUnifiedRadarSignedReceipt(payload, signature, hash, keyID, registry)
		if err != nil || identity.Network != network || identity.Target != target {
			writeAPIError(w, http.StatusServiceUnavailable, APICodeServiceUnavailable, "SOC signed receipt verification failed")
			return
		}
		var signed struct {
			Grade   string `json:"grade"`
			Verdict string `json:"verdict"`
		}
		if err := json.Unmarshal(payload, &signed); err != nil ||
			strings.TrimSpace(signed.Verdict) == "" {
			writeAPIError(w, http.StatusServiceUnavailable, APICodeServiceUnavailable, "SOC signed verdict payload unreadable")
			return
		}
		items = append(items, ownerSOCVerdictCandidate{
			Network: network, Target: target, Grade: signed.Grade, Verdict: signed.Verdict,
			EvidenceRef: immutableSOCVerdictRefPrefix + id + ":" + signature,
			RecordedAt:  recordedAt, Linked: linked,
		})
	}
	if err := rows.Err(); err != nil {
		writeAPIError(w, http.StatusServiceUnavailable, APICodeServiceUnavailable, "SOC signed candidate read unavailable")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":             true,
		"schema_version": "koschei.owner-soc-signed-candidates.v1",
		"boundary":       ownerSOCVerdictCandidateBoundary,
		"limit":          ownerSOCVerdictCandidateLimit,
		"count":          len(items),
		"items":          items,
	})
}
