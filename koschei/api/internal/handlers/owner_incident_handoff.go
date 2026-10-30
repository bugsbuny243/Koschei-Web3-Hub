package handlers

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"
)

// A handoff hash only detects changes to this serialized snapshot. It is not
// a signature, independent verification, on-chain proof, or an attestation.
const ownerIncidentHandoffSchema = "koschei.soc-incident-handoff.v1"
const ownerIncidentHandoffActionLimit = 500
const ownerIncidentHandoffMaxPayloadBytes = 128 << 10
const ownerIncidentHandoffMaxActionsBytes = 2 << 20

type ownerIncidentHandoffAction struct {
	ID         string          `json:"id"`
	ActionType string          `json:"action_type"`
	Actor      string          `json:"actor"`
	Summary    string          `json:"summary"`
	Payload    json.RawMessage `json:"payload"`
	CreatedAt  time.Time       `json:"created_at"`
}

type ownerIncidentHandoffContent struct {
	SchemaVersion string                       `json:"schema_version"`
	Incident      ownerIncidentRecord          `json:"incident"`
	Actions       []ownerIncidentHandoffAction `json:"actions"`
	Operations    *socHandoffOperations        `json:"operations,omitempty"`
}

func marshalOwnerIncidentHandoff(content ownerIncidentHandoffContent) ([]byte, string, error) {
	raw, err := json.Marshal(content)
	if err != nil {
		return nil, "", err
	}
	digest := sha256.Sum256(raw)
	return raw, hex.EncodeToString(digest[:]), nil
}

// The existing record scanner deliberately tolerates malformed references for
// ordinary incident views. A portable handoff must instead reject them.
func scanOwnerIncidentForHandoff(row incidentScanner) (ownerIncidentRecord, error) {
	var item ownerIncidentRecord
	var evidenceRaw, alertsRaw, dossiersRaw []byte
	var resolved sql.NullTime
	if err := row.Scan(
		&item.ID, &item.IncidentRef, &item.Title, &item.Severity, &item.Status,
		&item.Network, &item.Target, &item.Summary,
		&evidenceRaw, &alertsRaw, &dossiersRaw,
		&item.CreatedBy, &item.CreatedAt, &item.UpdatedAt, &resolved,
	); err != nil {
		return item, err
	}
	for _, target := range []struct {
		blob []byte
		dest *[]string
	}{
		{evidenceRaw, &item.EvidenceRefs},
		{alertsRaw, &item.AlertRefs},
		{dossiersRaw, &item.DossierRefs},
	} {
		if err := json.Unmarshal(target.blob, target.dest); err != nil || *target.dest == nil {
			return ownerIncidentRecord{}, errors.New("incident references cannot be decoded as string arrays")
		}
	}
	if resolved.Valid {
		t := resolved.Time
		item.ResolvedAt = &t
	}
	return item, nil
}

func (h *Handler) ownerIncidentHandoff(w http.ResponseWriter, r *http.Request, id string) {
	if h == nil || h.DB == nil {
		writeAPIError(w, http.StatusServiceUnavailable, APICodeServiceUnavailable, "Incident evidence handoff store is unavailable")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	r = r.WithContext(ctx)
	w.Header().Set("Cache-Control", "no-store")
	v2 := r.URL.Query().Get("handoff") == "2"
	db := h.DBRead
	if db == nil || v2 {
		db = h.DB
	}
	tx, err := db.BeginTx(r.Context(), &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	if err != nil {
		writeAPIError(w, http.StatusServiceUnavailable, APICodeServiceUnavailable, "Incident evidence handoff snapshot is unavailable")
		return
	}
	defer tx.Rollback()

	incident, err := scanOwnerIncidentForHandoff(tx.QueryRowContext(r.Context(), ownerIncidentSelectSQL()+" WHERE id=$1::uuid", id))
	if err == sql.ErrNoRows {
		writeAPIError(w, http.StatusNotFound, APICodeNotFound, "Incident was not found")
		return
	}
	if err != nil {
		writeAPIError(w, http.StatusServiceUnavailable, APICodeServiceUnavailable, "Incident handoff case could not be decoded")
		return
	}

	rows, err := tx.QueryContext(r.Context(), `
		SELECT id::text, action_type, actor, summary, payload, created_at
		FROM security_incident_actions WHERE incident_id=$1::uuid
		ORDER BY created_at ASC, id ASC LIMIT $2
	`, id, ownerIncidentHandoffActionLimit+1)
	if err != nil {
		writeAPIError(w, http.StatusServiceUnavailable, APICodeServiceUnavailable, "Incident handoff timeline is unavailable")
		return
	}
	actions := make([]ownerIncidentHandoffAction, 0)
	actionBytes := 0
	for rows.Next() {
		var action ownerIncidentHandoffAction
		var payload []byte
		if err := rows.Scan(&action.ID, &action.ActionType, &action.Actor, &action.Summary, &payload, &action.CreatedAt); err != nil {
			rows.Close()
			writeAPIError(w, http.StatusServiceUnavailable, APICodeServiceUnavailable, "Incident handoff action could not be decoded")
			return
		}
		if !json.Valid(payload) || len(payload) == 0 || payload[0] != '{' {
			rows.Close()
			writeAPIError(w, http.StatusServiceUnavailable, APICodeServiceUnavailable, "Incident handoff audit payload is invalid")
			return
		}
		actionBytes += len(payload) + len(action.Summary) + len(action.Actor)
		if len(payload) > ownerIncidentHandoffMaxPayloadBytes || actionBytes > ownerIncidentHandoffMaxActionsBytes {
			rows.Close()
			writeAPIError(w, http.StatusConflict, APICodeConflict, "Incident audit payload is too large for bounded handoff")
			return
		}
		action.Payload = append(json.RawMessage(nil), payload...)
		actions = append(actions, action)
	}
	rowErr := rows.Err()
	closeErr := rows.Close()
	if rowErr != nil || closeErr != nil {
		writeAPIError(w, http.StatusServiceUnavailable, APICodeServiceUnavailable, "Incident handoff timeline is incomplete")
		return
	}
	if len(actions) > ownerIncidentHandoffActionLimit {
		writeAPIError(w, http.StatusConflict, APICodeConflict, "Incident handoff needs pagination: more than 500 audit actions")
		return
	}
	content := ownerIncidentHandoffContent{
		SchemaVersion: ownerIncidentHandoffSchema, Incident: incident, Actions: actions,
	}
	if v2 {
		content.SchemaVersion = ownerIncidentHandoffSchemaV2
		content.Operations, err = readSOCHandoff(r.Context(), tx, id, time.Now().UTC())
		if err != nil {
			code, apiCode := http.StatusServiceUnavailable, APICodeServiceUnavailable
			if errors.Is(err, errSOCHandoffLimit) {
				code, apiCode = http.StatusConflict, APICodeConflict
			}
			writeAPIError(w, code, apiCode, "SOC handoff is unavailable, inconsistent or exceeds bounded export limits")
			return
		}
	}
	_, digest, err := marshalOwnerIncidentHandoff(content)
	if err != nil {
		writeAPIError(w, http.StatusServiceUnavailable, APICodeServiceUnavailable, "Incident handoff cannot be serialized")
		return
	}
	if err := tx.Commit(); err != nil {
		writeAPIError(w, http.StatusServiceUnavailable, APICodeServiceUnavailable, "Incident handoff consistent snapshot could not be committed")
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	hashContract := fmt.Sprintf("SHA-256 of compact encoding/json marshaled content; max %d actions; not a signature or evidence verification", ownerIncidentHandoffActionLimit)
	if v2 {
		hashContract = "SHA-256 of compact encoding/json marshaled content; max 500 incident actions and 500 SOC events; not a signature or evidence verification"
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":                    true,
		"schema_version":        content.SchemaVersion,
		"generated_at":          time.Now().UTC(),
		"content":               content,
		"content_sha256":        digest,
		"content_hash_contract": hashContract,
		"evidence_validation":   "not_performed_references_only",
		"actor_verification":    "not_performed_owner_operator_labels_only",
		"decision_boundary":     ownerIncidentDecisionBoundary,
	})
}
