package services

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

const PublicPromotionIngestVersion = "koschei.public-promotion-ingest.v1"

var ErrPublicPromotionSourceRejected = errors.New("public promotion source item rejected")

type PublicPromotionSourceBinding struct {
	Network      string `json:"network"`
	AssetRef     string `json:"asset_ref"`
	Platform     string `json:"platform"`
	PublicActor  string `json:"public_actor,omitempty"`
	CanonicalURL string `json:"canonical_url"`
	RelationRef  string `json:"relation_ref,omitempty"`
	Verified     bool   `json:"verified"`
}

type PublicPromotionCollectRequest struct {
	Binding PublicPromotionSourceBinding `json:"binding"`
	Since   time.Time                    `json:"since,omitempty"`
	Limit   int                          `json:"limit"`
}

type PublicPromotionSourceItem struct {
	ExternalID   string            `json:"external_id,omitempty"`
	CanonicalURL string            `json:"canonical_url"`
	PublicActor  string            `json:"public_actor,omitempty"`
	Visibility   string            `json:"visibility"`
	Excerpt      string            `json:"excerpt,omitempty"`
	PublishedAt  time.Time         `json:"published_at,omitempty"`
	ObservedAt   time.Time         `json:"observed_at,omitempty"`
	Metadata     map[string]string `json:"metadata,omitempty"`
}

type PublicPromotionSourceAdapter interface {
	Name() string
	Platform() string
	CollectPublicEvidence(context.Context, PublicPromotionCollectRequest) ([]PublicPromotionSourceItem, error)
}

type PublicPromotionIngestReport struct {
	Version         string   `json:"version"`
	Adapter         string   `json:"adapter"`
	Platform        string   `json:"platform"`
	Received        int      `json:"received"`
	Accepted        int      `json:"accepted"`
	Inserted        int      `json:"inserted"`
	Duplicate       int      `json:"duplicate"`
	Rejected        int      `json:"rejected"`
	ObservationRefs []string `json:"observation_refs"`
	Complete        bool     `json:"complete"`
	Limitations     []string `json:"limitations"`
}

func IngestPublicPromotionBinding(ctx context.Context, db *sql.DB, adapter PublicPromotionSourceAdapter, request PublicPromotionCollectRequest) (PublicPromotionIngestReport, error) {
	report := PublicPromotionIngestReport{
		Version:         PublicPromotionIngestVersion,
		ObservationRefs: []string{},
		Limitations: []string{
			"Only explicitly bound public sources are eligible for ingestion.",
			"Observed public promotion evidence does not independently prove shared control, manipulation, wrongdoing or identity.",
		},
	}
	if adapter == nil {
		return report, fmt.Errorf("%w: adapter is required", ErrPublicPromotionSourceRejected)
	}
	report.Adapter = strings.TrimSpace(adapter.Name())
	report.Platform = normalizePublicPromotionPlatform(adapter.Platform())
	if report.Adapter == "" || !validPublicPromotionPlatform(report.Platform) {
		return report, fmt.Errorf("%w: adapter name/platform is invalid", ErrPublicPromotionSourceRejected)
	}
	if db == nil {
		return report, ErrPublicPromotionEvidenceStoreDown
	}
	binding, err := normalizePublicPromotionSourceBinding(request.Binding)
	if err != nil {
		return report, err
	}
	if binding.Platform != report.Platform {
		return report, fmt.Errorf("%w: adapter platform %q does not match binding platform %q", ErrPublicPromotionSourceRejected, report.Platform, binding.Platform)
	}
	request.Binding = binding
	if request.Limit <= 0 || request.Limit > 250 {
		request.Limit = 100
	}
	items, err := adapter.CollectPublicEvidence(ctx, request)
	if err != nil {
		return report, fmt.Errorf("collect public promotion evidence with %s: %w", report.Adapter, err)
	}
	if len(items) > request.Limit {
		items = items[:request.Limit]
		report.Limitations = append(report.Limitations, "Adapter returned more items than requested; excess observations were ignored.")
	}
	report.Received = len(items)
	for _, item := range items {
		observation, normalizeErr := PublicPromotionObservationFromSourceItem(binding, report.Adapter, item)
		if normalizeErr != nil {
			report.Rejected++
			continue
		}
		report.Accepted++
		stored, inserted, persistErr := PersistPublicPromotionObservation(ctx, db, observation)
		if persistErr != nil {
			return report, persistErr
		}
		report.ObservationRefs = append(report.ObservationRefs, stored.ObservationRef)
		if inserted {
			report.Inserted++
		} else {
			report.Duplicate++
		}
	}
	report.Complete = true
	return report, nil
}

func PublicPromotionObservationFromSourceItem(binding PublicPromotionSourceBinding, adapterName string, item PublicPromotionSourceItem) (PublicPromotionObservation, error) {
	binding, err := normalizePublicPromotionSourceBinding(binding)
	if err != nil {
		return PublicPromotionObservation{}, err
	}
	adapterName = strings.ToLower(strings.TrimSpace(adapterName))
	if adapterName == "" {
		return PublicPromotionObservation{}, fmt.Errorf("%w: adapter name is required", ErrPublicPromotionSourceRejected)
	}
	if strings.ToLower(strings.TrimSpace(item.Visibility)) != "public" {
		return PublicPromotionObservation{}, fmt.Errorf("%w: source item visibility must be public", ErrPublicPromotionSourceRejected)
	}
	itemURL, itemDomain, err := canonicalizePublicPromotionURL(item.CanonicalURL)
	if err != nil {
		return PublicPromotionObservation{}, fmt.Errorf("%w: invalid item URL: %v", ErrPublicPromotionSourceRejected, err)
	}
	_, bindingDomain, err := canonicalizePublicPromotionURL(binding.CanonicalURL)
	if err != nil {
		return PublicPromotionObservation{}, fmt.Errorf("%w: invalid binding URL: %v", ErrPublicPromotionSourceRejected, err)
	}

	itemActor := normalizePublicPromotionActor(binding.Platform, item.PublicActor)
	bindingActor := normalizePublicPromotionActor(binding.Platform, binding.PublicActor)
	if bindingActor != "" && itemActor != bindingActor {
		return PublicPromotionObservation{}, fmt.Errorf("%w: public actor does not match verified source binding", ErrPublicPromotionSourceRejected)
	}
	if binding.Platform == "website" || binding.Platform == "other" {
		if itemDomain != bindingDomain {
			return PublicPromotionObservation{}, fmt.Errorf("%w: public website item escaped bound domain", ErrPublicPromotionSourceRejected)
		}
	}

	observedAt := item.ObservedAt.UTC()
	if observedAt.IsZero() {
		observedAt = time.Now().UTC()
	}
	externalID := strings.TrimSpace(item.ExternalID)
	sourceSuffix := externalID
	if sourceSuffix == "" {
		sourceSuffix = sha256Prefixed([]byte(itemURL))
	}
	sourceRef := "public-source:" + adapterName + ":" + sourceSuffix
	evidenceState := PublicPromotionObserved
	if binding.Verified {
		evidenceState = PublicPromotionVerified
	}
	return NormalizePublicPromotionObservation(PublicPromotionObservation{
		Network:       binding.Network,
		AssetRef:      binding.AssetRef,
		Platform:      binding.Platform,
		ExternalID:    externalID,
		CanonicalURL:  itemURL,
		PublicActor:   itemActor,
		SourceRef:     sourceRef,
		EvidenceState: evidenceState,
		Excerpt:       item.Excerpt,
		PublishedAt:   item.PublishedAt,
		ObservedAt:    observedAt,
		Metadata:      item.Metadata,
	})
}

func normalizePublicPromotionSourceBinding(binding PublicPromotionSourceBinding) (PublicPromotionSourceBinding, error) {
	binding.Network = strings.ToLower(strings.TrimSpace(binding.Network))
	binding.AssetRef = strings.TrimSpace(binding.AssetRef)
	binding.Platform = normalizePublicPromotionPlatform(binding.Platform)
	binding.PublicActor = normalizePublicPromotionActor(binding.Platform, binding.PublicActor)
	binding.RelationRef = strings.TrimSpace(binding.RelationRef)
	canonicalURL, _, err := canonicalizePublicPromotionURL(binding.CanonicalURL)
	if err != nil {
		return PublicPromotionSourceBinding{}, fmt.Errorf("%w: invalid source binding URL: %v", ErrPublicPromotionSourceRejected, err)
	}
	binding.CanonicalURL = canonicalURL
	if binding.Network == "" || binding.AssetRef == "" || !validPublicPromotionPlatform(binding.Platform) {
		return PublicPromotionSourceBinding{}, fmt.Errorf("%w: network, asset_ref and supported platform are required", ErrPublicPromotionSourceRejected)
	}
	if binding.Verified && binding.RelationRef == "" {
		return PublicPromotionSourceBinding{}, fmt.Errorf("%w: verified source binding requires canonical relation_ref", ErrPublicPromotionSourceRejected)
	}
	if binding.Platform != "website" && binding.Platform != "other" && binding.PublicActor == "" {
		return PublicPromotionSourceBinding{}, fmt.Errorf("%w: social source binding requires public_actor", ErrPublicPromotionSourceRejected)
	}
	return binding, nil
}
