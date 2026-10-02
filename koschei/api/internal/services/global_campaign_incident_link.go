package services

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

var (
	ErrGlobalCampaignIncidentDBRequired       = errors.New("global campaign incident link requires database")
	ErrGlobalCampaignIncidentIDRequired       = errors.New("global campaign incident link requires incident id")
	ErrGlobalCampaignIncidentCampaignRequired = errors.New("global campaign incident link requires campaign reference")
	ErrGlobalCampaignIncidentNotFound         = errors.New("security incident not found")
	ErrGlobalCampaignIncidentCampaignNotFound = errors.New("global campaign not found")
)

type GlobalCampaignIncidentLink struct {
	IncidentID           string    `json:"incident_id"`
	IncidentRef          string    `json:"incident_ref"`
	CampaignRef          string    `json:"campaign_ref"`
	CampaignRevision     int64     `json:"campaign_revision"`
	EvidenceHashSHA256   string    `json:"evidence_hash_sha256"`
	LinkedBy             string    `json:"linked_by"`
	LinkedAt             time.Time `json:"linked_at"`
	Inserted             bool      `json:"inserted"`
	Idempotent           bool      `json:"idempotent"`
	VerdictAuthority     bool      `json:"verdict_authority"`
	ContainmentAuthority bool      `json:"containment_authority"`
}

// LinkGlobalCampaignToIncident appends an exact campaign revision reference to
// an operator incident. It never changes incident status, campaign state,
// deterministic verdicts, grades, signatures, or containment state.
func LinkGlobalCampaignToIncident(ctx context.Context, db *sql.DB, incidentID, campaignRef, actor string) (GlobalCampaignIncidentLink, error) {
	if db == nil {
		return GlobalCampaignIncidentLink{}, ErrGlobalCampaignIncidentDBRequired
	}
	incidentID = strings.TrimSpace(incidentID)
	campaignRef = strings.TrimSpace(campaignRef)
	actor = strings.TrimSpace(actor)
	if incidentID == "" {
		return GlobalCampaignIncidentLink{}, ErrGlobalCampaignIncidentIDRequired
	}
	if campaignRef == "" {
		return GlobalCampaignIncidentLink{}, ErrGlobalCampaignIncidentCampaignRequired
	}
	if actor == "" {
		actor = "owner"
	}

	tx, err := db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return GlobalCampaignIncidentLink{}, err
	}
	defer func() { _ = tx.Rollback() }()

	lockKey := "global-campaign-incident:" + incidentID + ":" + campaignRef
	if _, err := tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1, 0))`, lockKey); err != nil {
		return GlobalCampaignIncidentLink{}, err
	}

	var incidentRef string
	if err := tx.QueryRowContext(ctx,
		`SELECT incident_ref FROM security_incident_cases WHERE id=$1::uuid FOR UPDATE`,
		incidentID,
	).Scan(&incidentRef); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return GlobalCampaignIncidentLink{}, ErrGlobalCampaignIncidentNotFound
		}
		return GlobalCampaignIncidentLink{}, err
	}

	var revision int64
	var evidenceHash string
	if err := tx.QueryRowContext(ctx,
		`SELECT revision, evidence_hash_sha256 FROM global_campaign_current WHERE campaign_ref=$1`,
		campaignRef,
	).Scan(&revision, &evidenceHash); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return GlobalCampaignIncidentLink{}, ErrGlobalCampaignIncidentCampaignNotFound
		}
		return GlobalCampaignIncidentLink{}, err
	}

	result := GlobalCampaignIncidentLink{
		IncidentID:         incidentID,
		IncidentRef:        incidentRef,
		CampaignRef:        campaignRef,
		CampaignRevision:   revision,
		EvidenceHashSHA256: evidenceHash,
		LinkedBy:           actor,
	}
	var linkedAt time.Time
	inserted := false
	err = tx.QueryRowContext(ctx, `
		INSERT INTO security_incident_campaign_links (
			incident_id, campaign_ref, campaign_revision, evidence_hash_sha256, linked_by
		) VALUES ($1::uuid,$2,$3,$4,$5)
		ON CONFLICT (incident_id, campaign_ref, campaign_revision) DO NOTHING
		RETURNING linked_at`,
		incidentID, campaignRef, revision, evidenceHash, actor,
	).Scan(&linkedAt)
	switch {
	case err == nil:
		inserted = true
	case errors.Is(err, sql.ErrNoRows):
		if err := tx.QueryRowContext(ctx, `
			SELECT linked_by, linked_at, evidence_hash_sha256
			FROM security_incident_campaign_links
			WHERE incident_id=$1::uuid AND campaign_ref=$2 AND campaign_revision=$3`,
			incidentID, campaignRef, revision,
		).Scan(&result.LinkedBy, &linkedAt, &result.EvidenceHashSHA256); err != nil {
			return GlobalCampaignIncidentLink{}, err
		}
		if result.EvidenceHashSHA256 != evidenceHash {
			return GlobalCampaignIncidentLink{}, fmt.Errorf("stored campaign incident link hash mismatch")
		}
	default:
		return GlobalCampaignIncidentLink{}, err
	}
	result.LinkedAt = linkedAt.UTC()
	result.Inserted = inserted
	result.Idempotent = !inserted

	if inserted {
		payload, err := json.Marshal(map[string]any{
			"campaign_ref":         campaignRef,
			"campaign_revision":    revision,
			"evidence_hash_sha256": evidenceHash,
			"authority_boundary":   "link_only_no_verdict_or_containment_authority",
		})
		if err != nil {
			return GlobalCampaignIncidentLink{}, err
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO security_incident_actions (incident_id,action_type,actor,summary,payload)
			VALUES ($1::uuid,'link_campaign',$2,$3,$4::jsonb)`,
			incidentID,
			actor,
			fmt.Sprintf("Linked Global Campaign %s revision %d", campaignRef, revision),
			string(payload),
		); err != nil {
			return GlobalCampaignIncidentLink{}, err
		}
	}

	if err := tx.Commit(); err != nil {
		return GlobalCampaignIncidentLink{}, err
	}
	return result, nil
}

func LoadGlobalCampaignIncidentLinks(ctx context.Context, db *sql.DB, incidentID string) ([]GlobalCampaignIncidentLink, error) {
	if db == nil {
		return nil, ErrGlobalCampaignIncidentDBRequired
	}
	incidentID = strings.TrimSpace(incidentID)
	if incidentID == "" {
		return nil, ErrGlobalCampaignIncidentIDRequired
	}
	rows, err := db.QueryContext(ctx, `
		SELECT l.incident_id::text, i.incident_ref, l.campaign_ref, l.campaign_revision,
		       l.evidence_hash_sha256, l.linked_by, l.linked_at
		FROM security_incident_campaign_links l
		JOIN security_incident_cases i ON i.id=l.incident_id
		WHERE l.incident_id=$1::uuid
		ORDER BY l.campaign_ref, l.campaign_revision`, incidentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []GlobalCampaignIncidentLink{}
	for rows.Next() {
		var item GlobalCampaignIncidentLink
		if err := rows.Scan(
			&item.IncidentID,
			&item.IncidentRef,
			&item.CampaignRef,
			&item.CampaignRevision,
			&item.EvidenceHashSHA256,
			&item.LinkedBy,
			&item.LinkedAt,
		); err != nil {
			return nil, err
		}
		item.LinkedAt = item.LinkedAt.UTC()
		out = append(out, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}
