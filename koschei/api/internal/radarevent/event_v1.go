package radarevent

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"

	"koschei/api/internal/networktarget"
	"koschei/api/internal/securityevidence"
)

const SchemaVersionV1 = "koschei.global-radar-event.v1"

const (
	KindBlock         = "block"
	KindTransaction   = "transaction"
	KindLog           = "log"
	KindAccount       = "account"
	KindAsset         = "asset"
	KindContract      = "contract"
	KindBridge        = "bridge"
	KindLiquidity     = "liquidity"
	KindNetworkHealth = "network_health"
)

type NativeReference struct {
	Kind  string `json:"kind"`
	Value string `json:"value"`
}

type Fact struct {
	Key            string `json:"key"`
	Value          string `json:"value"`
	Unit           string `json:"unit,omitempty"`
	EvidenceSHA256 string `json:"evidence_sha256,omitempty"`
}

type Event struct {
	SchemaVersion    string                         `json:"schema_version"`
	Producer         string                         `json:"producer"`
	Kind             string                         `json:"kind"`
	NetworkID        string                         `json:"network_id"`
	SubjectKind      string                         `json:"subject_kind"`
	SubjectID        string                         `json:"subject_id"`
	ObservedAtUnixMS int64                          `json:"observed_at_unix_ms"`
	State            securityevidence.EvidenceState `json:"evidence_state"`
	NativeRefs       []NativeReference              `json:"native_refs,omitempty"`
	SourceDigests    []string                       `json:"source_digests_sha256,omitempty"`
	Facts            []Fact                         `json:"facts,omitempty"`
	EventSHA256      string                         `json:"event_sha256"`
}

func (e Event) Canonical() (Event, error) {
	out := e
	out.SchemaVersion = strings.TrimSpace(out.SchemaVersion)
	if out.SchemaVersion == "" {
		out.SchemaVersion = SchemaVersionV1
	}
	if out.SchemaVersion != SchemaVersionV1 {
		return Event{}, fmt.Errorf("unsupported schema version %q", out.SchemaVersion)
	}

	out.Producer = strings.TrimSpace(out.Producer)
	out.Kind = strings.ToLower(strings.TrimSpace(out.Kind))
	out.NetworkID = strings.ToLower(strings.TrimSpace(out.NetworkID))
	out.SubjectKind = strings.ToLower(strings.TrimSpace(out.SubjectKind))
	out.SubjectID = strings.TrimSpace(out.SubjectID)
	out.EventSHA256 = ""

	if out.Producer == "" {
		return Event{}, errors.New("producer is required")
	}
	if !validKind(out.Kind) {
		return Event{}, fmt.Errorf("unsupported radar event kind %q", out.Kind)
	}
	if _, ok := networktarget.LookupNetwork(out.NetworkID); !ok {
		return Event{}, fmt.Errorf("network %q is not registered", out.NetworkID)
	}
	if !validSubjectKind(out.SubjectKind) || out.SubjectID == "" {
		return Event{}, errors.New("complete subject identity is required")
	}
	if out.ObservedAtUnixMS <= 0 {
		return Event{}, errors.New("observed_at_unix_ms must be positive")
	}
	switch out.State {
	case securityevidence.StateObserved, securityevidence.StateVerified, securityevidence.StateUnavailable, securityevidence.StateNotApplicable:
	default:
		return Event{}, fmt.Errorf("invalid evidence state %q", out.State)
	}

	sourceDigests, err := canonicalDigests(out.SourceDigests)
	if err != nil {
		return Event{}, err
	}
	out.SourceDigests = sourceDigests
	if (out.State == securityevidence.StateObserved || out.State == securityevidence.StateVerified) && len(out.SourceDigests) == 0 {
		return Event{}, errors.New("observed or verified radar event requires source digest")
	}

	nativeRefs, err := canonicalNativeRefs(out.NativeRefs)
	if err != nil {
		return Event{}, err
	}
	out.NativeRefs = nativeRefs

	facts, err := canonicalFacts(out.Facts)
	if err != nil {
		return Event{}, err
	}
	out.Facts = facts

	if (out.State == securityevidence.StateObserved || out.State == securityevidence.StateVerified) && len(out.NativeRefs) == 0 && len(out.Facts) == 0 {
		return Event{}, errors.New("observed or verified radar event requires native reference or fact")
	}

	return out, nil
}

func (e Event) Digest() (string, error) {
	canonical, err := e.Canonical()
	if err != nil {
		return "", err
	}
	payload, err := json.Marshal(canonical)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:]), nil
}

func (e Event) Seal() (Event, error) {
	canonical, err := e.Canonical()
	if err != nil {
		return Event{}, err
	}
	digest, err := canonical.Digest()
	if err != nil {
		return Event{}, err
	}
	canonical.EventSHA256 = digest
	return canonical, nil
}

func (e Event) Verify() error {
	expected := strings.ToLower(strings.TrimSpace(e.EventSHA256))
	if !validSHA256(expected) {
		return errors.New("event_sha256 is required and must be sha256")
	}
	actual, err := e.Digest()
	if err != nil {
		return err
	}
	if actual != expected {
		return errors.New("global radar event digest mismatch")
	}
	return nil
}

func validKind(kind string) bool {
	switch kind {
	case KindBlock, KindTransaction, KindLog, KindAccount, KindAsset, KindContract, KindBridge, KindLiquidity, KindNetworkHealth:
		return true
	default:
		return false
	}
}

func validSubjectKind(kind string) bool {
	switch kind {
	case "network", "block", "transaction", "log", "address", "account", "wallet", "contract", "program", "asset", "token", "pool", "bridge", "validator", "node", "miner", "protocol":
		return true
	default:
		return false
	}
}

func canonicalDigests(values []string) ([]string, error) {
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.ToLower(strings.TrimSpace(value))
		if !validSHA256(value) {
			return nil, fmt.Errorf("invalid source digest %q", value)
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	sort.Strings(out)
	return out, nil
}

func canonicalNativeRefs(values []NativeReference) ([]NativeReference, error) {
	seen := make(map[string]struct{}, len(values))
	out := make([]NativeReference, 0, len(values))
	for _, value := range values {
		value.Kind = strings.ToLower(strings.TrimSpace(value.Kind))
		value.Value = strings.TrimSpace(value.Value)
		if value.Kind == "" || value.Value == "" {
			return nil, errors.New("native reference kind and value are required")
		}
		if len(value.Kind) > 64 || len(value.Value) > 512 {
			return nil, errors.New("native reference exceeds size limit")
		}
		key := value.Kind + "\x00" + value.Value
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, value)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Kind != out[j].Kind {
			return out[i].Kind < out[j].Kind
		}
		return out[i].Value < out[j].Value
	})
	return out, nil
}

func canonicalFacts(values []Fact) ([]Fact, error) {
	seen := make(map[string]struct{}, len(values))
	out := make([]Fact, 0, len(values))
	for _, value := range values {
		value.Key = strings.ToLower(strings.TrimSpace(value.Key))
		value.Value = strings.TrimSpace(value.Value)
		value.Unit = strings.ToLower(strings.TrimSpace(value.Unit))
		value.EvidenceSHA256 = strings.ToLower(strings.TrimSpace(value.EvidenceSHA256))
		if value.Key == "" || value.Value == "" {
			return nil, errors.New("fact key and value are required")
		}
		if len(value.Key) > 128 || len(value.Value) > 4096 || len(value.Unit) > 64 {
			return nil, errors.New("fact exceeds size limit")
		}
		if value.EvidenceSHA256 != "" && !validSHA256(value.EvidenceSHA256) {
			return nil, fmt.Errorf("invalid fact evidence digest for %q", value.Key)
		}
		if _, ok := seen[value.Key]; ok {
			return nil, fmt.Errorf("duplicate fact key %q", value.Key)
		}
		seen[value.Key] = struct{}{}
		out = append(out, value)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Key < out[j].Key
	})
	return out, nil
}

func validSHA256(value string) bool {
	if len(value) != 64 {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}
