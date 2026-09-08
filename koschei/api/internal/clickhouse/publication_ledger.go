package clickhouse

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const (
	PublicationSchemaVersion = uint16(1)
	zeroUUID                 = "00000000-0000-0000-0000-000000000000"
	zeroSHA256               = "0000000000000000000000000000000000000000000000000000000000000000"
	maxPublicationRows       = uint64(100000)
)

var (
	dossierCaseRefRE = regexp.MustCompile(`^KD1-[a-z2-7]{32}$`)
	hex64RE          = regexp.MustCompile(`^[0-9a-f]{64}$`)
	uuidRE           = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)
)

type DossierBundleManifest struct {
	ManifestID                string
	CaseRef                   string
	BundleSHA256              string
	ArtifactURI               string
	ArtifactSHA256            string
	DossierVersion            string
	Network                   string
	TargetKind                string
	TargetID                  string
	VerdictGrade              string
	VerdictStatus             string
	RulesetVersion            string
	EvidenceRows              uint32
	VerifiedRows              uint32
	ObservedRows              uint32
	InferredRows              uint32
	UnknownRows               uint32
	AcceptancePass            uint32
	AcceptanceFail            uint32
	AcceptanceNotInvestigated uint32
	ProducedAt                time.Time
	ManifestSHA256            string
	SchemaVersion             uint16
}

type PublicationState struct {
	Status            string
	PublicTitle       string
	PublicSummary     string
	Featured          bool
	RedactionProfile  string
	PublishedBy       string
	ManifestID        string
	BundleSHA256      string
	ExposureStartedAt time.Time
}

type PublicationTransition struct {
	TransitionID             string
	CaseRef                  string
	Sequence                 uint64
	Action                   string
	Status                   string
	Actor                    string
	PublishedBy              string
	PublicTitle              string
	PublicSummary            string
	Featured                 bool
	RedactionProfile         string
	ManifestID               string
	BundleSHA256             string
	PreviousTransitionID     string
	PreviousTransitionSHA256 string
	TransitionSHA256         string
	OccurredAt               time.Time
	ExposureStartedAt        time.Time
	SchemaVersion            uint16
}

func NormalizeDossierBundleManifest(input DossierBundleManifest) (DossierBundleManifest, error) {
	manifest := input
	manifest.CaseRef = strings.TrimSpace(manifest.CaseRef)
	manifest.BundleSHA256 = normalizeHex64(manifest.BundleSHA256)
	manifest.ArtifactURI = strings.TrimSpace(manifest.ArtifactURI)
	manifest.ArtifactSHA256 = normalizeHex64(manifest.ArtifactSHA256)
	manifest.DossierVersion = strings.TrimSpace(manifest.DossierVersion)
	manifest.Network = strings.TrimSpace(manifest.Network)
	manifest.TargetKind = strings.TrimSpace(manifest.TargetKind)
	manifest.TargetID = strings.TrimSpace(manifest.TargetID)
	manifest.VerdictGrade = strings.TrimSpace(manifest.VerdictGrade)
	manifest.VerdictStatus = strings.TrimSpace(manifest.VerdictStatus)
	manifest.RulesetVersion = strings.TrimSpace(manifest.RulesetVersion)
	manifest.ProducedAt = manifest.ProducedAt.UTC().Truncate(time.Millisecond)
	manifest.SchemaVersion = PublicationSchemaVersion

	if !dossierCaseRefRE.MatchString(manifest.CaseRef) {
		return DossierBundleManifest{}, fmt.Errorf("invalid dossier case_ref")
	}
	if !hex64RE.MatchString(manifest.BundleSHA256) || !hex64RE.MatchString(manifest.ArtifactSHA256) {
		return DossierBundleManifest{}, fmt.Errorf("invalid dossier SHA-256")
	}
	if manifest.DossierVersion == "" || manifest.Network == "" || manifest.TargetKind == "" || manifest.TargetID == "" {
		return DossierBundleManifest{}, fmt.Errorf("dossier manifest identity fields are required")
	}
	if manifest.ProducedAt.IsZero() {
		return DossierBundleManifest{}, fmt.Errorf("dossier manifest produced_at is required")
	}
	if err := validateEvidenceArtifactURI(manifest.ArtifactURI); err != nil {
		return DossierBundleManifest{}, err
	}

	manifest.ManifestID = deterministicUUID("koschei-dossier-manifest-id-v1", manifest.CaseRef, manifest.BundleSHA256)
	manifest.ManifestSHA256 = dossierManifestSHA256(manifest)
	return manifest, nil
}

func BuildPublicationTransition(caseRef string, previous *PublicationTransition, next PublicationState, occurredAt time.Time) (PublicationTransition, error) {
	caseRef = strings.TrimSpace(caseRef)
	if !dossierCaseRefRE.MatchString(caseRef) {
		return PublicationTransition{}, fmt.Errorf("invalid dossier case_ref")
	}
	occurredAt = occurredAt.UTC().Truncate(time.Millisecond)
	if occurredAt.IsZero() {
		return PublicationTransition{}, fmt.Errorf("publication occurred_at is required")
	}

	next.Status = strings.ToLower(strings.TrimSpace(next.Status))
	next.PublicTitle = strings.TrimSpace(next.PublicTitle)
	next.PublicSummary = strings.TrimSpace(next.PublicSummary)
	next.RedactionProfile = strings.TrimSpace(next.RedactionProfile)
	next.PublishedBy = strings.TrimSpace(next.PublishedBy)
	next.ManifestID = strings.ToLower(strings.TrimSpace(next.ManifestID))
	next.BundleSHA256 = normalizeHex64(next.BundleSHA256)

	if next.Status != "draft" && next.Status != "public" && next.Status != "hidden" {
		return PublicationTransition{}, fmt.Errorf("publication status must be draft, public, or hidden")
	}
	if next.Featured && next.Status != "public" {
		return PublicationTransition{}, fmt.Errorf("only public publications can be featured")
	}
	if next.RedactionProfile == "" {
		return PublicationTransition{}, fmt.Errorf("publication redaction profile is required")
	}
	if !uuidRE.MatchString(next.ManifestID) || next.ManifestID == zeroUUID {
		return PublicationTransition{}, fmt.Errorf("publication manifest id is invalid")
	}
	if !hex64RE.MatchString(next.BundleSHA256) {
		return PublicationTransition{}, fmt.Errorf("publication bundle SHA-256 is invalid")
	}
	actor, err := publicationActor(next.PublishedBy)
	if err != nil {
		return PublicationTransition{}, err
	}

	sequence := uint64(1)
	previousID := zeroUUID
	previousHash := zeroSHA256
	action := publicationAction(nil, next)
	exposureStartedAt := time.Time{}
	if next.Status == "public" {
		exposureStartedAt = occurredAt
	}

	if previous != nil {
		if previous.CaseRef != caseRef {
			return PublicationTransition{}, fmt.Errorf("publication previous case_ref mismatch")
		}
		if previous.Sequence == ^uint64(0) {
			return PublicationTransition{}, fmt.Errorf("publication sequence overflow")
		}
		if err := validatePublicationTransitionSelf(*previous); err != nil {
			return PublicationTransition{}, fmt.Errorf("invalid previous publication transition: %w", err)
		}
		sequence = previous.Sequence + 1
		previousID = previous.TransitionID
		previousHash = previous.TransitionSHA256
		action = publicationAction(previous, next)
		exposureStartedAt = previous.ExposureStartedAt
		if next.Status == "public" && previous.Status != "public" {
			exposureStartedAt = occurredAt
		}
	}

	transition := PublicationTransition{
		CaseRef: caseRef, Sequence: sequence, Action: action, Status: next.Status, Actor: actor,
		PublishedBy: next.PublishedBy, PublicTitle: next.PublicTitle, PublicSummary: next.PublicSummary,
		Featured: next.Featured, RedactionProfile: next.RedactionProfile, ManifestID: next.ManifestID,
		BundleSHA256: next.BundleSHA256, PreviousTransitionID: previousID,
		PreviousTransitionSHA256: previousHash, OccurredAt: occurredAt,
		ExposureStartedAt: exposureStartedAt, SchemaVersion: PublicationSchemaVersion,
	}
	transition.TransitionID = publicationTransitionID(transition)
	transition.TransitionSHA256 = publicationTransitionSHA256(transition)
	return transition, nil
}

func ValidatePublicationTransitionChain(transitions []PublicationTransition) error {
	if len(transitions) == 0 {
		return nil
	}
	if uint64(len(transitions)) > maxPublicationRows {
		return fmt.Errorf("publication transition chain exceeds %d rows", maxPublicationRows)
	}
	var previous *PublicationTransition
	for i := range transitions {
		current := transitions[i]
		if err := validatePublicationTransitionSelf(current); err != nil {
			return fmt.Errorf("publication transition %d invalid: %w", i, err)
		}
		wantSequence := uint64(i + 1)
		if current.Sequence != wantSequence {
			return fmt.Errorf("publication transition sequence gap or fork: got=%d want=%d", current.Sequence, wantSequence)
		}
		nextState := PublicationState{
			Status: current.Status, PublicTitle: current.PublicTitle, PublicSummary: current.PublicSummary,
			Featured: current.Featured, RedactionProfile: current.RedactionProfile, PublishedBy: current.PublishedBy,
			ManifestID: current.ManifestID, BundleSHA256: current.BundleSHA256,
		}
		if previous == nil {
			if current.PreviousTransitionID != zeroUUID || current.PreviousTransitionSHA256 != zeroSHA256 {
				return fmt.Errorf("publication genesis transition has a non-zero predecessor")
			}
			if want := publicationAction(nil, nextState); current.Action != want {
				return fmt.Errorf("publication genesis action=%q want=%q", current.Action, want)
			}
			expectedExposure := time.Time{}
			if current.Status == "public" {
				expectedExposure = current.OccurredAt
			}
			if !sameOptionalMillisecondTime(current.ExposureStartedAt, expectedExposure) {
				return fmt.Errorf("publication genesis exposure start does not match state")
			}
		} else {
			if current.CaseRef != previous.CaseRef {
				return fmt.Errorf("publication transition case_ref changed within chain")
			}
			if current.PreviousTransitionID != previous.TransitionID || current.PreviousTransitionSHA256 != previous.TransitionSHA256 {
				return fmt.Errorf("publication transition predecessor mismatch")
			}
			if want := publicationAction(previous, nextState); current.Action != want {
				return fmt.Errorf("publication transition action=%q want=%q", current.Action, want)
			}
			expectedExposure := previous.ExposureStartedAt
			if current.Status == "public" && previous.Status != "public" {
				expectedExposure = current.OccurredAt
			}
			if !sameOptionalMillisecondTime(current.ExposureStartedAt, expectedExposure) {
				return fmt.Errorf("publication exposure start does not match transition semantics")
			}
		}
		previous = &transitions[i]
	}
	return nil
}

func LatestPublicationState(transitions []PublicationTransition) (PublicationState, bool, error) {
	if len(transitions) == 0 {
		return PublicationState{}, false, nil
	}
	if err := ValidatePublicationTransitionChain(transitions); err != nil {
		return PublicationState{}, false, err
	}
	current := transitions[len(transitions)-1]
	return PublicationState{
		Status: current.Status, PublicTitle: current.PublicTitle, PublicSummary: current.PublicSummary,
		Featured: current.Featured, RedactionProfile: current.RedactionProfile, PublishedBy: current.PublishedBy,
		ManifestID: current.ManifestID, BundleSHA256: current.BundleSHA256, ExposureStartedAt: current.ExposureStartedAt,
	}, true, nil
}

func dossierManifestSHA256(manifest DossierBundleManifest) string {
	fields := []string{
		manifest.CaseRef, manifest.BundleSHA256, manifest.ArtifactURI, manifest.ArtifactSHA256,
		manifest.DossierVersion, manifest.Network, manifest.TargetKind, manifest.TargetID,
		manifest.VerdictGrade, manifest.VerdictStatus, manifest.RulesetVersion,
		strconv.FormatUint(uint64(manifest.EvidenceRows), 10), strconv.FormatUint(uint64(manifest.VerifiedRows), 10),
		strconv.FormatUint(uint64(manifest.ObservedRows), 10), strconv.FormatUint(uint64(manifest.InferredRows), 10),
		strconv.FormatUint(uint64(manifest.UnknownRows), 10), strconv.FormatUint(uint64(manifest.AcceptancePass), 10),
		strconv.FormatUint(uint64(manifest.AcceptanceFail), 10), strconv.FormatUint(uint64(manifest.AcceptanceNotInvestigated), 10),
		strconv.FormatInt(manifest.ProducedAt.UnixMilli(), 10), strconv.FormatUint(uint64(PublicationSchemaVersion), 10),
	}
	return hashFields("koschei-dossier-manifest-v1", fields...)
}

func publicationTransitionID(transition PublicationTransition) string {
	digest := sha256.Sum256([]byte(publicationTransitionCanonical(transition)))
	return uuidFromDigest(digest)
}

func publicationTransitionSHA256(transition PublicationTransition) string {
	return hashFields("koschei-publication-transition-v1", transition.TransitionID, publicationTransitionCanonical(transition))
}

func publicationTransitionCanonical(transition PublicationTransition) string {
	exposureMillis := int64(0)
	if !transition.ExposureStartedAt.IsZero() {
		exposureMillis = transition.ExposureStartedAt.UTC().Truncate(time.Millisecond).UnixMilli()
	}
	fields := []string{
		transition.CaseRef, strconv.FormatUint(transition.Sequence, 10), transition.Action, transition.Status,
		transition.Actor, transition.PublishedBy, transition.PublicTitle, transition.PublicSummary,
		strconv.FormatBool(transition.Featured), transition.RedactionProfile, transition.ManifestID, transition.BundleSHA256,
		transition.PreviousTransitionID, transition.PreviousTransitionSHA256,
		strconv.FormatInt(transition.OccurredAt.UTC().Truncate(time.Millisecond).UnixMilli(), 10),
		strconv.FormatInt(exposureMillis, 10), strconv.FormatUint(uint64(PublicationSchemaVersion), 10),
	}
	var buffer bytes.Buffer
	writeLengthPrefixedFields(&buffer, fields)
	return buffer.String()
}

func validatePublicationTransitionSelf(transition PublicationTransition) error {
	if !dossierCaseRefRE.MatchString(transition.CaseRef) || transition.Sequence == 0 {
		return fmt.Errorf("invalid publication identity or sequence")
	}
	if !uuidRE.MatchString(strings.ToLower(transition.TransitionID)) || transition.TransitionID == zeroUUID {
		return fmt.Errorf("invalid transition id")
	}
	if !uuidRE.MatchString(strings.ToLower(transition.ManifestID)) || transition.ManifestID == zeroUUID {
		return fmt.Errorf("invalid manifest id")
	}
	if !hex64RE.MatchString(normalizeHex64(transition.BundleSHA256)) || !hex64RE.MatchString(normalizeHex64(transition.TransitionSHA256)) || !hex64RE.MatchString(normalizeHex64(transition.PreviousTransitionSHA256)) {
		return fmt.Errorf("invalid transition hash")
	}
	actor, err := publicationActor(transition.PublishedBy)
	if err != nil || transition.Actor != actor {
		return fmt.Errorf("publication actor does not match publisher")
	}
	if transition.Status != "draft" && transition.Status != "public" && transition.Status != "hidden" {
		return fmt.Errorf("invalid publication status")
	}
	switch transition.Action {
	case "publish", "hide", "draft", "update", "feature", "unfeature":
	default:
		return fmt.Errorf("invalid publication action")
	}
	if transition.Featured && transition.Status != "public" {
		return fmt.Errorf("featured transition is not public")
	}
	if transition.RedactionProfile == "" || transition.OccurredAt.IsZero() {
		return fmt.Errorf("publication transition metadata is incomplete")
	}
	if want := publicationTransitionID(transition); transition.TransitionID != want {
		return fmt.Errorf("publication transition id mismatch")
	}
	if want := publicationTransitionSHA256(transition); transition.TransitionSHA256 != want {
		return fmt.Errorf("publication transition hash mismatch")
	}
	return nil
}

func publicationAction(previous *PublicationTransition, next PublicationState) string {
	if previous == nil || previous.Status != next.Status {
		switch next.Status {
		case "public":
			return "publish"
		case "hidden":
			return "hide"
		default:
			return "draft"
		}
	}
	if previous.Featured != next.Featured {
		if next.Featured {
			return "feature"
		}
		return "unfeature"
	}
	return "update"
}

func publicationActor(publishedBy string) (string, error) {
	switch publishedBy {
	case "owner":
		return "owner", nil
	case "koschei-autopublish/v1":
		return "autopublish", nil
	default:
		return "", fmt.Errorf("unsupported publication authority")
	}
}

func validateEvidenceArtifactURI(raw string) error {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || parsed.Scheme == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return fmt.Errorf("invalid or credential-bearing evidence artifact URI")
	}
	switch parsed.Scheme {
	case "gdrive", "s3":
		if parsed.Host == "" || strings.Trim(parsed.Path, "/") == "" {
			return fmt.Errorf("evidence artifact URI is incomplete")
		}
	case "https":
		if parsed.Host == "" || parsed.Path == "" || parsed.Path == "/" {
			return fmt.Errorf("evidence artifact URI is incomplete")
		}
	default:
		return fmt.Errorf("unsupported evidence artifact URI scheme")
	}
	return nil
}

func deterministicUUID(domain string, fields ...string) string {
	digestHex := hashFields(domain, fields...)
	digest, _ := hex.DecodeString(digestHex)
	var value [16]byte
	copy(value[:], digest[:16])
	value[6] = (value[6] & 0x0f) | 0x50
	value[8] = (value[8] & 0x3f) | 0x80
	return formatUUID(value)
}

func uuidFromDigest(digest [32]byte) string {
	var value [16]byte
	copy(value[:], digest[:16])
	value[6] = (value[6] & 0x0f) | 0x50
	value[8] = (value[8] & 0x3f) | 0x80
	return formatUUID(value)
}

func formatUUID(value [16]byte) string {
	hexValue := hex.EncodeToString(value[:])
	return hexValue[0:8] + "-" + hexValue[8:12] + "-" + hexValue[12:16] + "-" + hexValue[16:20] + "-" + hexValue[20:32]
}

func hashFields(domain string, fields ...string) string {
	h := sha256.New()
	writeLengthPrefixedFields(h, append([]string{domain}, fields...))
	return hex.EncodeToString(h.Sum(nil))
}

func normalizeHex64(value string) string {
	return strings.ToLower(strings.TrimSpace(strings.TrimPrefix(value, "sha256:")))
}

func sameOptionalMillisecondTime(left, right time.Time) bool {
	if left.IsZero() || right.IsZero() {
		return left.IsZero() && right.IsZero()
	}
	return left.UTC().Truncate(time.Millisecond).Equal(right.UTC().Truncate(time.Millisecond))
}
