package services

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/url"
	"sort"
	"strings"
	"time"
	"unicode/utf8"
)

const PublicPromotionEvidenceSchemaVersion = "koschei.public-promotion-evidence.v1"

var (
	ErrPublicPromotionEvidenceInvalid    = errors.New("public promotion evidence is invalid")
	ErrPublicPromotionEvidenceStoreDown  = errors.New("public promotion evidence store unavailable")
)

type PublicPromotionEvidenceState string

const (
	PublicPromotionVerified   PublicPromotionEvidenceState = "verified"
	PublicPromotionObserved   PublicPromotionEvidenceState = "observed"
	PublicPromotionUnverified PublicPromotionEvidenceState = "unverified"
)

type PublicPromotionObservation struct {
	SchemaVersion          string                       `json:"schema_version"`
	ObservationRef         string                       `json:"observation_ref"`
	Network                string                       `json:"network"`
	AssetRef               string                       `json:"asset_ref"`
	Platform               string                       `json:"platform"`
	ExternalID             string                       `json:"external_id,omitempty"`
	CanonicalURL           string                       `json:"canonical_url"`
	CanonicalDomain        string                       `json:"canonical_domain"`
	PublicActor            string                       `json:"public_actor,omitempty"`
	SourceRef              string                       `json:"source_ref"`
	EvidenceState          PublicPromotionEvidenceState `json:"evidence_state"`
	ClaimFingerprintSHA256 string                       `json:"claim_fingerprint_sha256,omitempty"`
	ContentHashSHA256      string                       `json:"content_hash_sha256"`
	Excerpt                string                       `json:"excerpt,omitempty"`
	PublishedAt            time.Time                    `json:"published_at,omitempty"`
	ObservedAt             time.Time                    `json:"observed_at"`
	Metadata               map[string]string            `json:"metadata,omitempty"`

	VerdictAuthority       bool `json:"verdict_authority"`
	GradeAuthority         bool `json:"grade_authority"`
	SameOperatorClaim      bool `json:"same_operator_claim"`
	RealWorldIdentityClaim bool `json:"real_world_identity_claim"`
	WrongdoingClaim        bool `json:"wrongdoing_claim"`
}

type PublicPromotionCorrelationSignal struct {
	Kind            string   `json:"kind"`
	Key             string   `json:"key"`
	Assets          []string `json:"assets"`
	ObservationRefs []string `json:"observation_refs"`
	EvidenceState   string   `json:"evidence_state"`
	Reason          string   `json:"reason"`
}

type PublicPromotionCorrelationReport struct {
	SchemaVersion          string                             `json:"schema_version"`
	Signals                []PublicPromotionCorrelationSignal `json:"signals"`
	SameOperatorClaim      bool                               `json:"same_operator_claim"`
	RealWorldIdentityClaim bool                               `json:"real_world_identity_claim"`
	WrongdoingClaim        bool                               `json:"wrongdoing_claim"`
	Limitations            []string                           `json:"limitations"`
}

func NormalizePublicPromotionObservation(in PublicPromotionObservation) (PublicPromotionObservation, error) {
	out := in
	out.SchemaVersion = PublicPromotionEvidenceSchemaVersion
	out.Network = strings.ToLower(strings.TrimSpace(out.Network))
	out.AssetRef = strings.TrimSpace(out.AssetRef)
	out.Platform = normalizePublicPromotionPlatform(out.Platform)
	out.ExternalID = strings.TrimSpace(out.ExternalID)
	out.PublicActor = normalizePublicPromotionActor(out.Platform, out.PublicActor)
	out.SourceRef = strings.TrimSpace(out.SourceRef)
	out.Excerpt = truncateRunes(strings.Join(strings.Fields(out.Excerpt), " "), 480)
	out.Metadata = normalizePublicPromotionMetadata(out.Metadata)
	out.ObservedAt = out.ObservedAt.UTC()
	if !out.PublishedAt.IsZero() {
		out.PublishedAt = out.PublishedAt.UTC()
	}
	if out.EvidenceState == "" {
		out.EvidenceState = PublicPromotionObserved
	}

	canonicalURL, domain, err := canonicalizePublicPromotionURL(out.CanonicalURL)
	if err != nil {
		return PublicPromotionObservation{}, fmt.Errorf("%w: %v", ErrPublicPromotionEvidenceInvalid, err)
	}
	out.CanonicalURL = canonicalURL
	out.CanonicalDomain = domain

	if out.Network == "" || out.AssetRef == "" || out.SourceRef == "" || out.ObservedAt.IsZero() {
		return PublicPromotionObservation{}, fmt.Errorf("%w: network, asset_ref, source_ref and observed_at are required", ErrPublicPromotionEvidenceInvalid)
	}
	if !validPublicPromotionPlatform(out.Platform) {
		return PublicPromotionObservation{}, fmt.Errorf("%w: unsupported platform %q", ErrPublicPromotionEvidenceInvalid, out.Platform)
	}
	if !validPublicPromotionEvidenceState(out.EvidenceState) {
		return PublicPromotionObservation{}, fmt.Errorf("%w: unsupported evidence state %q", ErrPublicPromotionEvidenceInvalid, out.EvidenceState)
	}

	out.ClaimFingerprintSHA256 = ""
	if claim := normalizePublicPromotionClaim(out.Excerpt); claim != "" {
		out.ClaimFingerprintSHA256 = sha256Prefixed([]byte(claim))
	}

	out.VerdictAuthority = false
	out.GradeAuthority = false
	out.SameOperatorClaim = false
	out.RealWorldIdentityClaim = false
	out.WrongdoingClaim = false
	out.ContentHashSHA256 = hashPublicPromotionObservation(out)
	if out.ContentHashSHA256 == "" {
		return PublicPromotionObservation{}, fmt.Errorf("%w: could not hash observation", ErrPublicPromotionEvidenceInvalid)
	}
	digestHex := strings.TrimPrefix(out.ContentHashSHA256, "sha256:")
	out.ObservationRef = "KPUB1-" + strings.ToUpper(digestHex[:32])
	return out, nil
}

func PersistPublicPromotionObservation(ctx context.Context, db *sql.DB, in PublicPromotionObservation) (PublicPromotionObservation, bool, error) {
	if db == nil {
		return PublicPromotionObservation{}, false, ErrPublicPromotionEvidenceStoreDown
	}
	observation, err := NormalizePublicPromotionObservation(in)
	if err != nil {
		return PublicPromotionObservation{}, false, err
	}
	metadata, err := json.Marshal(observation.Metadata)
	if err != nil {
		return PublicPromotionObservation{}, false, fmt.Errorf("marshal public promotion metadata: %w", err)
	}
	var publishedAt any
	if !observation.PublishedAt.IsZero() {
		publishedAt = observation.PublishedAt
	}
	result, err := db.ExecContext(ctx, `
		INSERT INTO public.public_promotion_evidence (
			observation_ref, schema_version, network, asset_ref, platform, external_id,
			canonical_url, canonical_domain, public_actor, source_ref, evidence_state,
			claim_fingerprint_sha256, content_hash_sha256, excerpt, published_at,
			observed_at, metadata
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17::jsonb)
		ON CONFLICT (observation_ref) DO NOTHING`,
		observation.ObservationRef,
		observation.SchemaVersion,
		observation.Network,
		observation.AssetRef,
		observation.Platform,
		observation.ExternalID,
		observation.CanonicalURL,
		observation.CanonicalDomain,
		observation.PublicActor,
		observation.SourceRef,
		observation.EvidenceState,
		observation.ClaimFingerprintSHA256,
		observation.ContentHashSHA256,
		observation.Excerpt,
		publishedAt,
		observation.ObservedAt,
		metadata,
	)
	if err != nil {
		return PublicPromotionObservation{}, false, fmt.Errorf("persist public promotion evidence: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return PublicPromotionObservation{}, false, fmt.Errorf("public promotion rows affected: %w", err)
	}
	return observation, rows > 0, nil
}

func LoadPublicPromotionEvidence(ctx context.Context, db *sql.DB, network, assetRef string, limit int) ([]PublicPromotionObservation, error) {
	if db == nil {
		return nil, ErrPublicPromotionEvidenceStoreDown
	}
	network = strings.ToLower(strings.TrimSpace(network))
	assetRef = strings.TrimSpace(assetRef)
	if network == "" || assetRef == "" {
		return nil, fmt.Errorf("%w: network and asset_ref are required", ErrPublicPromotionEvidenceInvalid)
	}
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	rows, err := db.QueryContext(ctx, `
		SELECT schema_version, observation_ref, network, asset_ref, platform,
		       external_id, canonical_url, canonical_domain, public_actor, source_ref,
		       evidence_state, claim_fingerprint_sha256, content_hash_sha256, excerpt,
		       published_at, observed_at, metadata
		FROM public.public_promotion_evidence
		WHERE network=$1 AND asset_ref=$2
		ORDER BY observed_at DESC, observation_ref ASC
		LIMIT $3`, network, assetRef, limit)
	if err != nil {
		return nil, fmt.Errorf("load public promotion evidence: %w", err)
	}
	defer rows.Close()

	out := make([]PublicPromotionObservation, 0)
	for rows.Next() {
		var item PublicPromotionObservation
		var publishedAt sql.NullTime
		var metadata []byte
		if err := rows.Scan(
			&item.SchemaVersion,
			&item.ObservationRef,
			&item.Network,
			&item.AssetRef,
			&item.Platform,
			&item.ExternalID,
			&item.CanonicalURL,
			&item.CanonicalDomain,
			&item.PublicActor,
			&item.SourceRef,
			&item.EvidenceState,
			&item.ClaimFingerprintSHA256,
			&item.ContentHashSHA256,
			&item.Excerpt,
			&publishedAt,
			&item.ObservedAt,
			&metadata,
		); err != nil {
			return nil, fmt.Errorf("scan public promotion evidence: %w", err)
		}
		if publishedAt.Valid {
			item.PublishedAt = publishedAt.Time.UTC()
		}
		item.ObservedAt = item.ObservedAt.UTC()
		if len(metadata) > 0 {
			if err := json.Unmarshal(metadata, &item.Metadata); err != nil {
				return nil, fmt.Errorf("decode public promotion metadata: %w", err)
			}
		}
		out = append(out, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate public promotion evidence: %w", err)
	}
	return out, nil
}

// BuildPublicPromotionCorrelationReport finds public-source overlap across
// distinct assets. Every signal is watch-only inferred context: it is never
// same-operator, identity, wrongdoing, verdict or grade authority.
func BuildPublicPromotionCorrelationReport(observations []PublicPromotionObservation) PublicPromotionCorrelationReport {
	type group struct {
		assets map[string]struct{}
		refs   map[string]struct{}
	}
	groups := map[string]map[string]*group{
		"shared_public_actor":      {},
		"shared_public_domain":     {},
		"repeated_claim_fingerprint": {},
	}
	for _, raw := range observations {
		observation, err := NormalizePublicPromotionObservation(raw)
		if err != nil || observation.EvidenceState == PublicPromotionUnverified {
			continue
		}
		keys := map[string]string{
			"shared_public_actor":      observation.Platform + ":" + observation.PublicActor,
			"shared_public_domain":     observation.CanonicalDomain,
			"repeated_claim_fingerprint": observation.ClaimFingerprintSHA256,
		}
		for kind, key := range keys {
			if key == "" || key == observation.Platform+":" {
				continue
			}
			g := groups[kind][key]
			if g == nil {
				g = &group{assets: map[string]struct{}{}, refs: map[string]struct{}{}}
				groups[kind][key] = g
			}
			g.assets[observation.AssetRef] = struct{}{}
			g.refs[observation.ObservationRef] = struct{}{}
		}
	}

	report := PublicPromotionCorrelationReport{
		SchemaVersion: "koschei.public-promotion-correlation.v1",
		Limitations: []string{
			"public-source overlap is correlation context, not proof of common control",
			"coordination indicators do not establish fraud, manipulation or criminal identity",
			"private groups or messages are outside this evidence contract unless separately lawfully authorized",
		},
	}
	for kind, keyed := range groups {
		for key, g := range keyed {
			assets := setToSortedStrings(g.assets)
			if len(assets) < 2 {
				continue
			}
			refs := setToSortedStrings(g.refs)
			reason := "same public evidence attribute observed across distinct assets"
			switch kind {
			case "shared_public_actor":
				reason = "same public account/handle observed in promotion evidence for distinct assets"
			case "shared_public_domain":
				reason = "same public domain observed in promotion evidence for distinct assets"
			case "repeated_claim_fingerprint":
				reason = "same normalized public claim fingerprint observed across distinct assets"
			}
			report.Signals = append(report.Signals, PublicPromotionCorrelationSignal{
				Kind:            kind,
				Key:             key,
				Assets:          assets,
				ObservationRefs: refs,
				EvidenceState:   "inferred",
				Reason:          reason,
			})
		}
	}
	sort.Slice(report.Signals, func(i, j int) bool {
		if report.Signals[i].Kind == report.Signals[j].Kind {
			return report.Signals[i].Key < report.Signals[j].Key
		}
		return report.Signals[i].Kind < report.Signals[j].Kind
	})
	return report
}

// PublicPromotionCampaignMaterial exposes a public observation to the existing
// Global Campaign evidence model without treating the public actor as an on-chain
// actor and without manufacturing a relation anchor. The observation cannot
// automatically merge unrelated campaigns merely because a handle/domain repeats.
func PublicPromotionCampaignMaterial(observation PublicPromotionObservation) (GlobalCampaignMaterializerInput, error) {
	normalized, err := NormalizePublicPromotionObservation(observation)
	if err != nil {
		return GlobalCampaignMaterializerInput{}, err
	}
	verified := 0
	observed := 0
	switch normalized.EvidenceState {
	case PublicPromotionVerified:
		verified = 1
	case PublicPromotionObserved:
		observed = 1
	}
	return GlobalCampaignMaterializerInput{
		ObservedAt:           normalized.ObservedAt,
		Networks:             []string{normalized.Network},
		Subjects:             []string{normalized.AssetRef},
		Assets:               []string{normalized.AssetRef},
		ObservationRefs:      []string{normalized.ObservationRef},
		VerifiedAnchorCount:  verified,
		ObservedAnchorCount:  observed,
		RulesetVersion:       "koschei.public-promotion-correlation.v1",
	}, nil
}

func canonicalizePublicPromotionURL(raw string) (string, string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", "", errors.New("canonical_url is required")
	}
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return "", "", errors.New("canonical_url must be an absolute public URL")
	}
	u.Scheme = strings.ToLower(u.Scheme)
	if u.Scheme != "http" && u.Scheme != "https" {
		return "", "", errors.New("canonical_url must use http or https")
	}
	if u.User != nil {
		return "", "", errors.New("canonical_url must not contain credentials")
	}
	hostname := strings.ToLower(strings.TrimSuffix(u.Hostname(), "."))
	if hostname == "" {
		return "", "", errors.New("canonical_url host is required")
	}
	port := u.Port()
	if (u.Scheme == "http" && port == "80") || (u.Scheme == "https" && port == "443") {
		port = ""
	}
	if port != "" {
		u.Host = net.JoinHostPort(hostname, port)
	} else {
		u.Host = hostname
	}
	u.Fragment = ""
	query := u.Query()
	for key := range query {
		lower := strings.ToLower(key)
		if strings.HasPrefix(lower, "utm_") || lower == "fbclid" || lower == "gclid" || lower == "mc_cid" || lower == "mc_eid" {
			query.Del(key)
		}
	}
	u.RawQuery = query.Encode()
	if u.Path == "" {
		u.Path = "/"
	}
	return u.String(), hostname, nil
}

func normalizePublicPromotionPlatform(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "twitter", "twitter/x", "x.com":
		return "x"
	case "telegram", "tg":
		return "telegram"
	case "youtube", "yt":
		return "youtube"
	case "reddit":
		return "reddit"
	case "discord":
		return "discord"
	case "website", "web", "site":
		return "website"
	case "other":
		return "other"
	default:
		return strings.ToLower(strings.TrimSpace(raw))
	}
}

func validPublicPromotionPlatform(platform string) bool {
	switch platform {
	case "x", "telegram", "youtube", "reddit", "discord", "website", "other":
		return true
	default:
		return false
	}
}

func validPublicPromotionEvidenceState(state PublicPromotionEvidenceState) bool {
	switch state {
	case PublicPromotionVerified, PublicPromotionObserved, PublicPromotionUnverified:
		return true
	default:
		return false
	}
}

func normalizePublicPromotionActor(platform, actor string) string {
	actor = strings.TrimSpace(actor)
	if actor == "" {
		return ""
	}
	if strings.HasPrefix(actor, "@") {
		actor = strings.TrimPrefix(actor, "@")
	}
	actor = strings.Join(strings.Fields(actor), " ")
	if platform != "website" && platform != "other" {
		actor = strings.ToLower(actor)
	}
	return truncateRunes(actor, 160)
}

func normalizePublicPromotionClaim(raw string) string {
	return strings.ToLower(strings.Join(strings.Fields(strings.TrimSpace(raw)), " "))
}

func normalizePublicPromotionMetadata(in map[string]string) map[string]string {
	if len(in) == 0 {
		return map[string]string{}
	}
	out := make(map[string]string, len(in))
	for rawKey, rawValue := range in {
		key := truncateRunes(strings.ToLower(strings.TrimSpace(rawKey)), 80)
		value := truncateRunes(strings.TrimSpace(rawValue), 240)
		if key == "" || value == "" {
			continue
		}
		out[key] = value
	}
	return out
}

func hashPublicPromotionObservation(observation PublicPromotionObservation) string {
	type hashInput struct {
		SchemaVersion   string            `json:"schema_version"`
		Network         string            `json:"network"`
		AssetRef        string            `json:"asset_ref"`
		Platform        string            `json:"platform"`
		ExternalID      string            `json:"external_id"`
		CanonicalURL    string            `json:"canonical_url"`
		CanonicalDomain string            `json:"canonical_domain"`
		PublicActor     string            `json:"public_actor"`
		SourceRef       string            `json:"source_ref"`
		EvidenceState   string            `json:"evidence_state"`
		Excerpt         string            `json:"excerpt"`
		PublishedAt     string            `json:"published_at"`
		ObservedAt      string            `json:"observed_at"`
		Metadata        map[string]string `json:"metadata"`
	}
	publishedAt := ""
	if !observation.PublishedAt.IsZero() {
		publishedAt = observation.PublishedAt.UTC().Format(time.RFC3339Nano)
	}
	payload, err := json.Marshal(hashInput{
		SchemaVersion:   observation.SchemaVersion,
		Network:         observation.Network,
		AssetRef:        observation.AssetRef,
		Platform:        observation.Platform,
		ExternalID:      observation.ExternalID,
		CanonicalURL:    observation.CanonicalURL,
		CanonicalDomain: observation.CanonicalDomain,
		PublicActor:     observation.PublicActor,
		SourceRef:       observation.SourceRef,
		EvidenceState:   string(observation.EvidenceState),
		Excerpt:         observation.Excerpt,
		PublishedAt:     publishedAt,
		ObservedAt:      observation.ObservedAt.UTC().Format(time.RFC3339Nano),
		Metadata:        observation.Metadata,
	})
	if err != nil {
		return ""
	}
	return sha256Prefixed(payload)
}

func sha256Prefixed(data []byte) string {
	digest := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(digest[:])
}

func truncateRunes(value string, limit int) string {
	if limit <= 0 || utf8.RuneCountInString(value) <= limit {
		return value
	}
	runes := []rune(value)
	return string(runes[:limit])
}

func setToSortedStrings(in map[string]struct{}) []string {
	out := make([]string, 0, len(in))
	for value := range in {
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}
