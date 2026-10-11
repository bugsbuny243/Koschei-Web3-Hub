// Package radaradapter defines the read-only collection boundary for chain adapters.
// It deliberately does not issue risk grades: the existing unified verdict engine
// remains the sole authority for grade decisions.
package radaradapter

import (
	"context"
	"errors"
	"strings"
	"time"
)

// Target preserves the caller's exact identifier and the explicit network scope.
type Target struct {
	Network string
	Kind    string
	Raw     string
}

// SubjectIdentity binds a subject to a chain's native identity primitive.
type SubjectIdentity struct {
	Network       string
	IdentityKind  string
	IdentityValue string
	NativeID      string
}

// EvidenceRow is a transport-neutral collection row. Integration with the
// existing services.IntelligenceEvidence type belongs to the dispatcher
// boundary, avoiding an import cycle from services to this package.
type EvidenceRow struct {
	ID         string
	SubjectID  string
	Status     string
	Source     string
	ObservedAt time.Time
	Height     string
	RawHash    string
	Attributes map[string]any
}

type Limitation struct {
	Code   string
	Reason string
}

type Capability struct {
	Networks         []string
	TargetKinds      []string
	DataSources      []string
	RequiresArchival bool
	Fixture          string
}

type Adapter interface {
	Supports(target Target) bool
	Collect(ctx context.Context, target Target) ([]EvidenceRow, []Limitation, error)
	Subject() SubjectIdentity
	Capability() Capability
}

var ErrDeclaredNotExecutable = errors.New("radar adapter declared but not executable")

// ValidateEvidence rejects rows missing reproducibility fields. No missing
// provider or raw response digest can be treated as verified evidence.
func ValidateEvidence(subject SubjectIdentity, rows []EvidenceRow) error {
	if strings.TrimSpace(subject.Network) == "" || strings.TrimSpace(subject.IdentityKind) == "" ||
		strings.TrimSpace(subject.IdentityValue) == "" || strings.TrimSpace(subject.NativeID) == "" {
		return errors.New("incomplete chain subject identity")
	}
	seen := make(map[string]struct{}, len(rows))
	for _, row := range rows {
		if _, exists := seen[row.ID]; exists {
			return errors.New("duplicate evidence row id")
		}
		seen[row.ID] = struct{}{}
		switch strings.ToLower(strings.TrimSpace(row.Status)) {
		case "verified", "observed", "inferred", "unverified":
		default:
			return errors.New("unknown evidence status")
		}

		if strings.TrimSpace(row.ID) == "" || strings.TrimSpace(row.SubjectID) == "" ||
			strings.TrimSpace(row.Source) == "" || strings.TrimSpace(row.Height) == "" ||
			strings.TrimSpace(row.RawHash) == "" || row.ObservedAt.IsZero() {
			return errors.New("incomplete reproducible evidence row")
		}
	}
	return nil
}
