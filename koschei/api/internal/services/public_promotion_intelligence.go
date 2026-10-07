package services

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
)

const PublicPromotionIntelligenceVersion = "koschei.public-promotion-intelligence.v1"

type PublicPromotionIntelligence struct {
	Version                 string                           `json:"version"`
	Network                 string                           `json:"network"`
	AssetRef                string                           `json:"asset_ref"`
	Available               bool                             `json:"available"`
	Complete                bool                             `json:"complete"`
	Status                  string                           `json:"status"`
	CurrentObservationCount int                              `json:"current_observation_count"`
	RelatedObservationCount int                              `json:"related_observation_count"`
	CurrentObservations     []PublicPromotionObservation     `json:"current_observations"`
	RelatedObservations     []PublicPromotionObservation     `json:"related_observations"`
	Correlation             PublicPromotionCorrelationReport `json:"correlation"`
	VerdictAuthority        bool                             `json:"verdict_authority"`
	GradeAuthority          bool                             `json:"grade_authority"`
	SameOperatorClaim       bool                             `json:"same_operator_claim"`
	RealWorldIdentityClaim  bool                             `json:"real_world_identity_claim"`
	WrongdoingClaim         bool                             `json:"wrongdoing_claim"`
	MarketManipulationClaim bool                             `json:"market_manipulation_claim"`
	Limitations             []string                         `json:"limitations"`
}

func NewPublicPromotionIntelligenceUnavailable(network, assetRef, status, limitation string) PublicPromotionIntelligence {
	if strings.TrimSpace(status) == "" {
		status = "source_unavailable"
	}
	out := PublicPromotionIntelligence{
		Version:             PublicPromotionIntelligenceVersion,
		Network:             strings.ToLower(strings.TrimSpace(network)),
		AssetRef:            strings.TrimSpace(assetRef),
		Status:              status,
		CurrentObservations: []PublicPromotionObservation{},
		RelatedObservations: []PublicPromotionObservation{},
		Correlation:         BuildPublicPromotionCorrelationReport(nil),
		Limitations: []string{
			"Public-source promotion evidence is optional context and never changes the deterministic ARVIS verdict by itself.",
			"Public account/domain/claim overlap does not prove common control, manipulation, fraud, criminal identity or coordinated intent.",
		},
	}
	if strings.TrimSpace(limitation) != "" {
		out.Limitations = append(out.Limitations, strings.TrimSpace(limitation))
	}
	return out
}

// LoadPublicPromotionIntelligence loads the current asset's retained public
// promotion evidence, then expands only through explicit public evidence keys
// already observed for that asset. Social platforms correlate by public account
// and normalized claim fingerprint; domain correlation is reserved for project
// websites/other public web sources so common hosts such as x.com or t.me cannot
// widen an investigation by themselves.
func LoadPublicPromotionIntelligence(ctx context.Context, db *sql.DB, network, assetRef string, currentLimit, relatedLimit int) (PublicPromotionIntelligence, error) {
	network = strings.ToLower(strings.TrimSpace(network))
	assetRef = strings.TrimSpace(assetRef)
	if currentLimit <= 0 || currentLimit > 500 {
		currentLimit = 100
	}
	if relatedLimit <= 0 || relatedLimit > 1000 {
		relatedLimit = 500
	}
	out := NewPublicPromotionIntelligenceUnavailable(network, assetRef, "no_public_promotion_evidence", "")
	out.Complete = true
	if network == "" || assetRef == "" {
		out.Complete = false
		out.Status = "invalid_target"
		return out, nil
	}
	if db == nil {
		out.Complete = false
		out.Status = "source_unavailable"
		out.Limitations = append(out.Limitations, "Public promotion evidence store is unavailable.")
		return out, nil
	}

	current, err := LoadPublicPromotionEvidence(ctx, db, network, assetRef, currentLimit)
	if err != nil {
		return NewPublicPromotionIntelligenceUnavailable(network, assetRef, "source_unavailable", "Current-asset public promotion evidence could not be read."), err
	}
	out.CurrentObservations = current
	out.CurrentObservationCount = len(current)
	if len(current) == 0 {
		return out, nil
	}

	related, err := loadRelatedPublicPromotionEvidence(ctx, db, network, assetRef, relatedLimit)
	if err != nil {
		out.Complete = false
		out.Status = "partial_related_evidence_unavailable"
		out.Available = true
		out.Limitations = append(out.Limitations, "Cross-asset public promotion correlation could not be read; current-asset public evidence remains available.")
		return out, err
	}
	out.RelatedObservations = related
	out.RelatedObservationCount = len(related)
	all := append(append([]PublicPromotionObservation{}, current...), related...)
	out.Correlation = BuildPublicPromotionCorrelationReport(all)
	out.Available = true
	out.Status = "current_public_promotion_evidence_observed"
	if len(out.Correlation.Signals) > 0 {
		out.Status = "cross_asset_public_promotion_overlap_observed"
	}
	return out, nil
}

func loadRelatedPublicPromotionEvidence(ctx context.Context, db *sql.DB, network, assetRef string, limit int) ([]PublicPromotionObservation, error) {
	rows, err := db.QueryContext(ctx, `
		WITH current_keys AS (
			SELECT DISTINCT
				platform,
				NULLIF(btrim(public_actor),'') AS public_actor,
				CASE
					WHEN platform IN ('website','other') THEN NULLIF(btrim(canonical_domain),'')
					ELSE NULL
				END AS canonical_domain,
				NULLIF(btrim(claim_fingerprint_sha256),'') AS claim_fingerprint_sha256
			FROM public.public_promotion_evidence
			WHERE network=$1
			  AND asset_ref=$2
			  AND evidence_state IN ('verified','observed')
		), related_refs AS (
			SELECT DISTINCT e.observation_ref
			FROM public.public_promotion_evidence e
			JOIN current_keys k ON (
				(k.public_actor IS NOT NULL AND e.platform=k.platform AND e.public_actor=k.public_actor)
				OR (
					k.canonical_domain IS NOT NULL
					AND e.platform IN ('website','other')
					AND e.canonical_domain=k.canonical_domain
				)
				OR (k.claim_fingerprint_sha256 IS NOT NULL AND e.claim_fingerprint_sha256=k.claim_fingerprint_sha256)
			)
			WHERE e.network=$1
			  AND e.asset_ref<>$2
			  AND e.evidence_state IN ('verified','observed')
		)
		SELECT e.schema_version, e.observation_ref, e.network, e.asset_ref, e.platform,
		       e.external_id, e.canonical_url, e.canonical_domain, e.public_actor, e.source_ref,
		       e.evidence_state, e.claim_fingerprint_sha256, e.content_hash_sha256, e.excerpt,
		       e.published_at, e.observed_at, e.metadata
		FROM public.public_promotion_evidence e
		JOIN related_refs r ON r.observation_ref=e.observation_ref
		ORDER BY e.observed_at DESC, e.observation_ref ASC
		LIMIT $3`, network, assetRef, limit)
	if err != nil {
		return nil, fmt.Errorf("load related public promotion evidence: %w", err)
	}
	defer rows.Close()

	out := make([]PublicPromotionObservation, 0)
	for rows.Next() {
		var item PublicPromotionObservation
		var publishedAt sql.NullTime
		var metadata []byte
		if err := rows.Scan(
			&item.SchemaVersion, &item.ObservationRef, &item.Network, &item.AssetRef, &item.Platform,
			&item.ExternalID, &item.CanonicalURL, &item.CanonicalDomain, &item.PublicActor, &item.SourceRef,
			&item.EvidenceState, &item.ClaimFingerprintSHA256, &item.ContentHashSHA256, &item.Excerpt,
			&publishedAt, &item.ObservedAt, &metadata,
		); err != nil {
			return nil, fmt.Errorf("scan related public promotion evidence: %w", err)
		}
		if publishedAt.Valid {
			item.PublishedAt = publishedAt.Time.UTC()
		}
		item.ObservedAt = item.ObservedAt.UTC()
		if len(metadata) > 0 {
			if err := json.Unmarshal(metadata, &item.Metadata); err != nil {
				return nil, fmt.Errorf("decode related public promotion metadata: %w", err)
			}
		}
		item.VerdictAuthority = false
		item.GradeAuthority = false
		item.SameOperatorClaim = false
		item.RealWorldIdentityClaim = false
		item.WrongdoingClaim = false
		out = append(out, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate related public promotion evidence: %w", err)
	}
	return out, nil
}
