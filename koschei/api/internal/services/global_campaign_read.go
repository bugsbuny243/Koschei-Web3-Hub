package services

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

// ListCurrentGlobalCampaigns returns validated current campaign projections in
// most-recent-observation order. Corrupt metadata/payload pairs fail closed;
// callers never receive a partially trusted campaign list.
func ListCurrentGlobalCampaigns(ctx context.Context, db *sql.DB, limit int) ([]GlobalCampaign, error) {
	if db == nil {
		return nil, ErrGlobalCampaignStoreUnavailable
	}
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}
	rows, err := db.QueryContext(ctx, `
		SELECT campaign_ref,revision,schema_version,state,first_observed_at,last_observed_at,
		       ruleset_version,evidence_hash_sha256,canonical_payload
		FROM global_campaign_current
		ORDER BY last_observed_at DESC, campaign_ref ASC
		LIMIT $1`, limit)
	if err != nil {
		return nil, fmt.Errorf("list global campaign current projections: %w", err)
	}
	defer rows.Close()

	items := make([]GlobalCampaign, 0, limit)
	for rows.Next() {
		var campaignRef string
		var stored globalCampaignStoredRow
		if err := rows.Scan(
			&campaignRef,
			&stored.Revision,
			&stored.SchemaVersion,
			&stored.State,
			&stored.FirstObservedAt,
			&stored.LastObservedAt,
			&stored.RulesetVersion,
			&stored.EvidenceHashSHA256,
			&stored.CanonicalPayload,
		); err != nil {
			return nil, fmt.Errorf("scan global campaign current projection: %w", err)
		}
		campaign, err := decodeStoredGlobalCampaign(strings.TrimSpace(campaignRef), stored)
		if err != nil {
			return nil, err
		}
		items = append(items, campaign)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate global campaign current projections: %w", err)
	}
	return items, nil
}
