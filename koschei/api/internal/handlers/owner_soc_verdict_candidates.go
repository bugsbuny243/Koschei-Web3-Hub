package handlers

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
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

// ownerSOCVerdictCursor encodes the exclusive (recorded_at,id) keyset boundary.
// It is not an authentication token. Owner auth and independent receipt
// signature checks apply to every retrieved page, including older decisions.
func ownerSOCVerdictCursor(at time.Time, id string) string {
	return base64.RawURLEncoding.EncodeToString([]byte(at.UTC().Format(time.RFC3339Nano) + "|" + id))
}

func parseOwnerSOCVerdictCursor(raw string) (time.Time, string, error) {
	if raw == "" || len(raw) > 160 {
		return time.Time{}, "", fmt.Errorf("invalid historical receipt cursor")
	}
	decoded, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil || base64.RawURLEncoding.EncodeToString(decoded) != raw {
		return time.Time{}, "", fmt.Errorf("noncanonical historical receipt cursor")
	}
	pieces := strings.Split(string(decoded), "|")
	if len(pieces) != 2 || !unifiedSOCVerdictRefUUID.MatchString(pieces[1]) {
		return time.Time{}, "", fmt.Errorf("invalid historical receipt cursor fields")
	}
	at, err := time.Parse(time.RFC3339Nano, pieces[0])
	if err != nil || at.UTC().Format(time.RFC3339Nano) != pieces[0] {
		return time.Time{}, "", fmt.Errorf("invalid historical receipt cursor timestamp")
	}
	return at, pieces[1], nil
}

// ownerSOCVerdictPageFilter rejects ambiguous and unbounded filter inputs
// before any database access. Empty filters preserve the original 20-newest
// owner triage behavior. Scoped queries require an exact network/target pair.
func ownerSOCVerdictPageFilter(rawQuery string) (network, target string, beforeAt, beforeID any, err error) {
	params, err := url.ParseQuery(rawQuery)
	if err != nil {
		return "", "", nil, nil, err
	}
	for key, values := range params {
		if (key != "network" && key != "target" && key != "cursor") || len(values) != 1 {
			return "", "", nil, nil, fmt.Errorf("unsupported or duplicate historical receipt filter")
		}
	}
	network, target = params.Get("network"), params.Get("target")
	if len(network) > 64 || len(target) > 256 ||
		network != strings.TrimSpace(network) || target != strings.TrimSpace(target) ||
		strings.ContainsAny(network+target, "\r\n\x00") || (network == "") != (target == "") {
		return "", "", nil, nil, fmt.Errorf("invalid historical receipt subject")
	}
	if values, provided := params["cursor"]; provided {
		if len(values) != 1 {
			return "", "", nil, nil, fmt.Errorf("duplicate historical receipt cursor")
		}
		t, id, parseErr := parseOwnerSOCVerdictCursor(values[0])
		if parseErr != nil {
			return "", "", nil, nil, parseErr
		}
		beforeAt, beforeID = t, id
	}
	return network, target, beforeAt, beforeID, nil
}

// OwnerSOCVerdictCandidates lists only independently verified immutable ARVIS
// receipts under the existing ownerOnly authorization boundary. It supports
// keyset pagination for historical receipts, unlike the mutable fingerprint
// projection. It cannot create incidents, raise case severity or enforce.
func (h *Handler) OwnerSOCVerdictCandidates(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	networkFilter, targetFilter, beforeAt, beforeID, err := ownerSOCVerdictPageFilter(r.URL.RawQuery)
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, APICodeInvalidRequest, "Invalid SOC signed receipt history filter")
		return
	}
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
		  AND ($1::text = '' OR receipt.network = $1)
		  AND ($2::text = '' OR receipt.target_id = $2)
		  AND ($3::timestamptz IS NULL OR
		       (receipt.recorded_at,receipt.id) < ($3::timestamptz,$4::uuid))
		ORDER BY receipt.recorded_at DESC,receipt.id DESC
		LIMIT $5
	`, networkFilter, targetFilter, beforeAt, beforeID, ownerSOCVerdictCandidateLimit+1)
	if err != nil {
		writeAPIError(w, http.StatusServiceUnavailable, APICodeServiceUnavailable, "SOC signed verdict candidate read unavailable")
		return
	}
	defer rows.Close()
	items := make([]ownerSOCVerdictCandidate, 0, ownerSOCVerdictCandidateLimit)
	var nextCursor string
	var hasMore bool
	for rows.Next() {
		// The extra row establishes only that another page exists. No unverifed
		// receipt is returned, and the next page must authenticate it itself.
		if len(items) == ownerSOCVerdictCandidateLimit {
			hasMore = true
			break
		}
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
		nextCursor = ownerSOCVerdictCursor(recordedAt, id)
	}
	if err := rows.Err(); err != nil {
		writeAPIError(w, http.StatusServiceUnavailable, APICodeServiceUnavailable, "SOC signed candidate read unavailable")
		return
	}
	if !hasMore {
		nextCursor = ""
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":             true,
		"schema_version": "koschei.owner-soc-signed-candidates.v1",
		"boundary":       ownerSOCVerdictCandidateBoundary,
		"limit":          ownerSOCVerdictCandidateLimit,
		"count":          len(items),
		"items":          items,
		"has_more":       hasMore,
		"next_cursor":    nextCursor,
	})
}
