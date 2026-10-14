// Package radaradapter resolves collectors to the same evidence rows as the verdict core.
// Registration is contingent on offline conformance, never on a catalog label.
package radaradapter

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

type Factory func(Target, Transport) (Adapter, error)
type Registry struct {
	mu      sync.RWMutex
	entries map[string]entry
}
type entry struct {
	capability Capability
	factory    Factory
}

func NewRegistry() *Registry { return &Registry{entries: make(map[string]entry)} }

// Conform checks a recorded, offline run. No boolean "conformance passed" claim is accepted.
func Conform(ctx context.Context, adapter Adapter, target Target) error {
	if adapter == nil || !adapter.Supports(target) {
		return errors.New("fixture target unsupported")
	}
	rows, _, err := adapter.Collect(ctx, target)
	if err != nil {
		return errors.New("failure must carry a limitation")
	}
	s := adapter.Subject()
	if s.Network != target.Network || s.NativeID == "" || s.ChainIdentity == "" {
		return errors.New("fixture subject identity missing")
	}
	if len(rows) == 0 {
		return errors.New("empty collection must be UNKNOWN with limitations")
	}
	for _, r := range rows {
		if err := ValidateRow(r, s); err != nil {
			return err
		}
	}
	return nil
}
func ValidateRow(r EvidenceRow, s SubjectIdentity) error {
	if err := ValidateEvidence(CollectorIdentity(s), []EvidenceRow{r}); err != nil {
		return err
	}
	if n, err := strconv.ParseInt(r.Height, 10, 64); err != nil || n < 0 {
		return errors.New("invalid evidence height")
	}
	if r.RawHash != r.Attributes["response_sha256"] || r.Height != r.Attributes["height"] {
		return errors.New("provenance field mismatch")
	}
	if r.Network != s.Network || r.SubjectID != s.NativeID || r.Relation == "" || r.ID == "" || r.ObservedAt.IsZero() {
		return errors.New("unbound evidence row")
	}
	switch r.Status {
	case "verified", "observed", "inferred", "unverified":
	default:
		return errors.New("invalid evidence status")
	}
	for _, k := range []string{"chain_identity", "height", "provider_host", "response_sha256"} {
		v, ok := r.Attributes[k].(string)
		if !ok || v == "" {
			return fmt.Errorf("missing provenance %s", k)
		}
	}
	if r.Attributes["chain_identity"] != s.ChainIdentity {
		return errors.New("evidence chain mismatch")
	}
	h, _ := r.Attributes["response_sha256"].(string)
	if b, e := hex.DecodeString(h); e != nil || len(b) != 32 {
		return errors.New("invalid response digest")
	}
	if r.Attributes["critical"] == true && r.Status == "verified" {
		hosts, ok := r.Attributes["confirmation_hosts"].([]string)
		if !ok || len(hosts) < 2 || hosts[0] == hosts[1] {
			return errors.New("critical evidence needs independent confirmation")
		}
	}
	return nil
}
func (r *Registry) Register(factory Factory, target Target, fixture Transport) error {
	adapter, err := factory(target, fixture)
	if err != nil {
		return err
	}
	if err = Conform(context.Background(), adapter, target); err != nil {
		return err
	}
	c := adapter.Capability()
	if c.Network != target.Network || c.Fixture == "" {
		return errors.New("invalid capability")
	}
	c.Status = "executable"
	c.Networks = []string{c.Network}
	c.TargetKinds = append([]string(nil), c.TargetClasses...)
	c.DataSources = append([]string(nil), c.Sources...)
	c.RequiresArchival = len(c.ArchivalRequirements) > 0
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.entries[c.Network]; ok {
		return errors.New("duplicate network registration")
	}
	r.entries[c.Network] = entry{c, factory}
	return nil
}
func (r *Registry) Capabilities() []Capability {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := []Capability{}
	for _, e := range r.entries {
		c := e.capability
		c.TargetClasses = append([]string(nil), c.TargetClasses...)
		c.Sources = append([]string(nil), c.Sources...)
		c.ArchivalRequirements = append([]string(nil), c.ArchivalRequirements...)
		out = append(out, c)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Network < out[j].Network })
	return out
}
func (r *Registry) Supports(t Target) bool {
	r.mu.RLock()
	e, ok := r.entries[t.Network]
	r.mu.RUnlock()
	if !ok {
		return false
	}
	for _, k := range e.capability.TargetClasses {
		if k == t.Kind {
			return true
		}
	}
	return false
}

type Collection struct {
	Subject     SubjectIdentity `json:"subject"`
	Status      string          `json:"status"`
	Evidence    []EvidenceRow   `json:"evidence"`
	Limitations []Limitation    `json:"limitations"`
	Requests    int             `json:"requests"`
}

func (r *Registry) Run(ctx context.Context, t Target, transport Transport) Collection {
	out := Collection{Subject: SubjectIdentity{Network: t.Network, NativeID: t.Raw}, Status: "declared_not_executable", Evidence: []EvidenceRow{}, Limitations: []Limitation{}}
	r.mu.RLock()
	e, ok := r.entries[t.Network]
	r.mu.RUnlock()
	if !ok || !r.Supports(t) {
		out.Limitations = append(out.Limitations, Limitation{"collector_missing", "No conformance-tested collector for this network and target class."})
		return out
	}
	a, err := e.factory(t, transport)
	if err != nil {
		out.Status = "invalid_target"
		out.Limitations = append(out.Limitations, Limitation{"invalid_target", err.Error()})
		return out
	}
	rows, limits, err := a.Collect(ctx, t)
	out.Subject = a.Subject()
	out.Limitations = append(out.Limitations, limits...)
	out.Status = "observed"
	seen := map[string]bool{}
	for _, row := range rows {
		if seen[row.ID] {
			out.Limitations = append(out.Limitations, Limitation{"duplicate_evidence", "Duplicate evidence ID rejected."})
			continue
		}
		seen[row.ID] = true
		if validation := ValidateRow(row, out.Subject); validation != nil {
			out.Limitations = append(out.Limitations, Limitation{"invalid_evidence", validation.Error()})
			continue
		}
		if e.capability.EvidenceOnly && row.Status == "verified" {
			row.Status = "observed"
		}
		out.Evidence = append(out.Evidence, row)
	}
	if err != nil {
		out.Limitations = append(out.Limitations, Limitation{"collection_failed", "Provider collection failed; no safety conclusion is available."})
	}
	if len(out.Evidence) == 0 {
		out.Status = "insufficient_evidence"
	} else if len(out.Limitations) > 0 {
		out.Status = "partial_investigation"
	}
	if counter, ok := transport.(interface{ Count() int }); ok {
		out.Requests = counter.Count()
	}
	return out
}
func Digest(b []byte) string { h := sha256.Sum256(b); return hex.EncodeToString(h[:]) }
func NewRow(s SubjectIdentity, relation string, response Response, height string, facts map[string]any) EvidenceRow {
	if strings.HasPrefix(height, "0x") {
		if n, err := strconv.ParseUint(height[2:], 16, 63); err == nil {
			height = strconv.FormatUint(n, 10)
		}
	}
	m := map[string]any{"chain_identity": s.ChainIdentity, "height": height, "provider_host": response.Host, "response_sha256": Digest(response.Raw)}
	m["adapter_observe_only"] = true
	if s.Network == "offchain" {
		m["height_kind"] = "not_applicable"
	}
	for k, v := range facts {
		if _, exists := m[k]; !exists {
			m[k] = v
		}
	}
	observedAt := time.Now().UTC()
	factsJSON, _ := json.Marshal(facts)
	id := Digest([]byte(strings.Join([]string{s.Network, s.NativeID, relation, height, response.Host, Digest(response.Raw), Digest(factsJSON), observedAt.Format(time.RFC3339Nano)}, "|")))
	return EvidenceRow{ID: id, Network: s.Network, SubjectID: s.NativeID, Relation: relation, Status: "observed", Height: height, RawHash: Digest(response.Raw), Source: response.Host, ObservedAt: observedAt, Attributes: m}
}
func Decode(r Response, out any) error {
	if !json.Valid(r.Raw) {
		return errors.New("invalid provider JSON")
	}
	return json.Unmarshal(r.Raw, out)
}

// Describe validates syntax without collecting or claiming existence/ownership.
func (r *Registry) Describe(t Target) (SubjectIdentity, error) {
	r.mu.RLock()
	e, ok := r.entries[t.Network]
	r.mu.RUnlock()
	if !ok || !r.Supports(t) {
		return SubjectIdentity{}, errors.New("target class not executable")
	}
	a, err := e.factory(t, nil)
	if err != nil {
		return SubjectIdentity{}, err
	}
	return a.Subject(), nil
}
