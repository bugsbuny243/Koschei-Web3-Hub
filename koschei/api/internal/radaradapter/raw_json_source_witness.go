package radaradapter

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

const RawJSONSourceWitnessVersion = "koschei.radaradapter.raw-json-source-witness.v1"
const RawJSONSourceWitnessMaxBytes = 2 << 20

// RawJSONObservation is a collector-local input. Its source response bytes
// must originate from the actual RPC/API reply; never from a rebuilt report.
// The raw response is deliberately not retained in the returned witness.
type RawJSONObservation struct {
	Row          EvidenceRow
	ResponseJSON []byte
}

// RawJSONSourceWitness binds transport bytes and declared row identity into a
// reproducible commitment. It is not chain inclusion, independent consensus,
// signer verification, a verified EvidenceRow, or a verdict.
type RawJSONSourceWitness struct {
	SchemaVersion                 string    `json:"schema_version"`
	RowID                         string    `json:"row_id"`
	Network                       string    `json:"network"`
	SubjectID                     string    `json:"subject_id"`
	ReportedHeight                string    `json:"reported_height"`
	ObservedAt                    time.Time `json:"observed_at"`
	ResponseSHA256                string    `json:"response_sha256"`
	DeclaredAttributesSHA256      string    `json:"declared_attributes_sha256"`
	CommitmentSHA256              string    `json:"commitment_sha256"`
	IntegrityStatus               string    `json:"integrity_status"`
	HeightIndependentlyVerified   bool      `json:"height_independently_verified"`
	AttributesIndependentlyProven bool      `json:"attributes_independently_proven"`
	ChainEvidenceVerified         bool      `json:"chain_evidence_verified"`
	VerdictBound                  bool      `json:"verdict_bound"`
}

type rawJSONSourceCommitment struct {
	SchemaVersion            string `json:"schema_version"`
	Network                  string `json:"network"`
	NativeID                 string `json:"native_id"`
	IdentityKind             string `json:"identity_kind"`
	IdentityValue            string `json:"identity_value"`
	RowID                    string `json:"row_id"`
	Relation                 string `json:"relation"`
	SourceSHA256             string `json:"source_sha256"`
	ReportedHeight           string `json:"reported_height"`
	ObservedAtRFC3339Nano    string `json:"observed_at"`
	ResponseSHA256           string `json:"response_sha256"`
	DeclaredAttributesSHA256 string `json:"declared_attributes_sha256"`
}

// BindRawJSONSourceWitnesses validates collector-supplied byte integrity,
// subject scope and declared height syntax. It never upgrades verification,
// changes verdicts or writes evidence/alert/incident storage.
func BindRawJSONSourceWitnesses(subject SubjectIdentity, observations []RawJSONObservation) ([]RawJSONSourceWitness, error) {
	if len(observations) == 0 || len(observations) > 256 {
		return nil, errors.New("source witness requires between 1 and 256 observations")
	}
	rows := make([]EvidenceRow, 0, len(observations))
	for _, observation := range observations {
		rows = append(rows, observation.Row)
	}
	if err := ValidateEvidence(subject, rows); err != nil {
		return nil, fmt.Errorf("collector evidence incomplete: %w", err)
	}

	out := make([]RawJSONSourceWitness, 0, len(observations))
	for _, observation := range observations {
		row := observation.Row
		if !strings.EqualFold(strings.TrimSpace(row.Network), strings.TrimSpace(subject.Network)) ||
			strings.TrimSpace(row.SubjectID) != strings.TrimSpace(subject.NativeID) {
			return nil, errors.New("evidence network or native subject identity mismatch")
		}
		if strings.ToLower(strings.TrimSpace(row.Status)) != "observed" {
			return nil, errors.New("raw response integrity cannot promote declared verified or inferred evidence")
		}
		if strings.TrimSpace(row.Relation) == "" || len(row.ID) > 512 {
			return nil, errors.New("source observation relation or id invalid")
		}
		if len(observation.ResponseJSON) == 0 || len(observation.ResponseJSON) > RawJSONSourceWitnessMaxBytes ||
			!json.Valid(observation.ResponseJSON) {
			return nil, errors.New("source response is missing, oversized or not valid JSON")
		}
		height, err := strconv.ParseUint(row.Height, 10, 64)
		if err != nil || height == 0 || strconv.FormatUint(height, 10) != row.Height {
			return nil, errors.New("source reported height must be a canonical positive integer")
		}
		hash := sha256.Sum256(observation.ResponseJSON)
		expected := hex.EncodeToString(hash[:])
		if len(row.RawHash) != 64 || strings.ToLower(row.RawHash) != row.RawHash ||
			subtle.ConstantTimeCompare([]byte(row.RawHash), []byte(expected)) != 1 {
			return nil, errors.New("source response SHA-256 does not match observed bytes")
		}
		attributes, err := json.Marshal(row.Attributes)
		if err != nil {
			return nil, fmt.Errorf("declared observation attributes cannot be encoded: %w", err)
		}
		attrHash := sha256.Sum256(attributes)
		sourceHash := sha256.Sum256([]byte(row.Source))
		material := rawJSONSourceCommitment{
			SchemaVersion:            RawJSONSourceWitnessVersion,
			Network:                  subject.Network,
			NativeID:                 subject.NativeID,
			IdentityKind:             subject.IdentityKind,
			IdentityValue:            subject.IdentityValue,
			RowID:                    row.ID,
			Relation:                 row.Relation,
			SourceSHA256:             hex.EncodeToString(sourceHash[:]),
			ReportedHeight:           row.Height,
			ObservedAtRFC3339Nano:    row.ObservedAt.UTC().Format(time.RFC3339Nano),
			ResponseSHA256:           expected,
			DeclaredAttributesSHA256: hex.EncodeToString(attrHash[:]),
		}
		bytes, err := json.Marshal(material)
		if err != nil {
			return nil, fmt.Errorf("source witness cannot be serialized: %w", err)
		}
		commitment := sha256.Sum256(bytes)
		out = append(out, RawJSONSourceWitness{
			SchemaVersion:                 RawJSONSourceWitnessVersion,
			RowID:                         row.ID,
			Network:                       subject.Network,
			SubjectID:                     subject.NativeID,
			ReportedHeight:                row.Height,
			ObservedAt:                    row.ObservedAt.UTC(),
			ResponseSHA256:                expected,
			DeclaredAttributesSHA256:      hex.EncodeToString(attrHash[:]),
			CommitmentSHA256:              hex.EncodeToString(commitment[:]),
			IntegrityStatus:               "collector_raw_json_bytes_match_declared_digest_only",
			HeightIndependentlyVerified:   false,
			AttributesIndependentlyProven: false,
			ChainEvidenceVerified:         false,
			VerdictBound:                  false,
		})
	}
	return out, nil
}
