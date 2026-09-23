package networktarget

import (
	"fmt"
	"math"
	"regexp"
	"strings"
	"time"
)

const NetworkTelemetrySchemaVersion = "koschei.network-telemetry.v1"

var (
	networkTelemetryASN         = regexp.MustCompile(`^AS[0-9]{1,10}$`)
	networkTelemetryCountryCode = regexp.MustCompile(`^[A-Z]{2}$`)
)

type NetworkTelemetryInput struct {
	NetworkID      string
	SubjectKind    string
	SubjectID      string
	Source         string
	ObservedAt     time.Time
	EvidenceStatus string
	ClientFamily   string
	StakeSharePct  *float64
	HashSharePct   *float64
	ASN            string
	CountryCode    string
	LocationSource string
}

type NetworkTelemetryObservation struct {
	SchemaVersion   string    `json:"schema_version"`
	Network         Network   `json:"network"`
	SubjectKind     string    `json:"subject_kind"`
	SubjectID       string    `json:"subject_id"`
	Source          string    `json:"source"`
	ObservedAt      time.Time `json:"observed_at"`
	EvidenceStatus  string    `json:"evidence_status"`
	ClientFamily    string    `json:"client_family,omitempty"`
	StakeSharePct   *float64  `json:"stake_share_pct,omitempty"`
	HashSharePct    *float64  `json:"hash_share_pct,omitempty"`
	ASN             string    `json:"asn,omitempty"`
	CountryCode     string    `json:"country_code,omitempty"`
	LocationSource  string    `json:"location_source,omitempty"`
	MissingEvidence []string  `json:"missing_evidence"`
}

// NormalizeNetworkTelemetry converts a source observation into a bounded,
// network-scoped evidence record. It never invents location, client, stake or
// hash-rate data. Precise end-user/device coordinates are intentionally not
// part of this contract.
func NormalizeNetworkTelemetry(input NetworkTelemetryInput) (NetworkTelemetryObservation, error) {
	network, ok := LookupNetwork(input.NetworkID)
	if !ok {
		return NetworkTelemetryObservation{}, fmt.Errorf("network_not_registered")
	}

	subjectKind := strings.ToLower(strings.TrimSpace(input.SubjectKind))
	switch subjectKind {
	case "network", "node", "validator", "miner":
	default:
		return NetworkTelemetryObservation{}, fmt.Errorf("network_telemetry_subject_kind_invalid")
	}

	subjectID := strings.TrimSpace(input.SubjectID)
	if subjectKind == "network" && subjectID == "" {
		subjectID = network.ID
	}
	if subjectID == "" || len(subjectID) > 256 {
		return NetworkTelemetryObservation{}, fmt.Errorf("network_telemetry_subject_id_invalid")
	}

	source := strings.TrimSpace(input.Source)
	if source == "" || len(source) > 256 {
		return NetworkTelemetryObservation{}, fmt.Errorf("network_telemetry_source_required")
	}
	if input.ObservedAt.IsZero() {
		return NetworkTelemetryObservation{}, fmt.Errorf("network_telemetry_observed_at_required")
	}

	evidenceStatus := strings.ToLower(strings.TrimSpace(input.EvidenceStatus))
	switch evidenceStatus {
	case "observed", "verified":
	default:
		return NetworkTelemetryObservation{}, fmt.Errorf("network_telemetry_evidence_status_invalid")
	}

	stakeShare, err := normalizeNetworkTelemetryPercent(input.StakeSharePct)
	if err != nil {
		return NetworkTelemetryObservation{}, fmt.Errorf("network_telemetry_stake_share_invalid")
	}
	hashShare, err := normalizeNetworkTelemetryPercent(input.HashSharePct)
	if err != nil {
		return NetworkTelemetryObservation{}, fmt.Errorf("network_telemetry_hash_share_invalid")
	}

	asn := strings.ToUpper(strings.TrimSpace(input.ASN))
	if asn != "" && !networkTelemetryASN.MatchString(asn) {
		return NetworkTelemetryObservation{}, fmt.Errorf("network_telemetry_asn_invalid")
	}

	countryCode := strings.ToUpper(strings.TrimSpace(input.CountryCode))
	locationSource := strings.TrimSpace(input.LocationSource)
	if countryCode != "" {
		if !networkTelemetryCountryCode.MatchString(countryCode) {
			return NetworkTelemetryObservation{}, fmt.Errorf("network_telemetry_country_code_invalid")
		}
		if locationSource == "" {
			return NetworkTelemetryObservation{}, fmt.Errorf("network_telemetry_location_source_required")
		}
	}

	clientFamily := strings.TrimSpace(input.ClientFamily)
	missing := make([]string, 0, 4)
	if clientFamily == "" {
		missing = append(missing, "client_family")
	}
	if stakeShare == nil && hashShare == nil {
		missing = append(missing, "consensus_contribution")
	}
	if asn == "" {
		missing = append(missing, "asn")
	}
	if countryCode == "" {
		missing = append(missing, "country_code")
	}

	return NetworkTelemetryObservation{
		SchemaVersion:   NetworkTelemetrySchemaVersion,
		Network:         network,
		SubjectKind:     subjectKind,
		SubjectID:       subjectID,
		Source:          source,
		ObservedAt:      input.ObservedAt.UTC(),
		EvidenceStatus:  evidenceStatus,
		ClientFamily:    clientFamily,
		StakeSharePct:   stakeShare,
		HashSharePct:    hashShare,
		ASN:             asn,
		CountryCode:     countryCode,
		LocationSource:  locationSource,
		MissingEvidence: missing,
	}, nil
}

func normalizeNetworkTelemetryPercent(value *float64) (*float64, error) {
	if value == nil {
		return nil, nil
	}
	if math.IsNaN(*value) || math.IsInf(*value, 0) || *value < 0 || *value > 100 {
		return nil, fmt.Errorf("percentage_out_of_range")
	}
	normalized := *value
	return &normalized, nil
}
