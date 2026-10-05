// Package sentinelclient connects Koschei Web3 to Koschei Sentinel through the
// public Sentinel case contract. It is observe-only: Sentinel output is
// commentary and can never mutate or replace the signed ARVIS verdict.
package sentinelclient

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"koschei/api/internal/outboundhttp"
)

const (
	caseSchemaVersion = "sentinel.case.v1"
	maxResponseBytes  = 256 << 10
)

type Config struct {
	Enabled bool
	BaseURL string
	Token   string
}

func ConfigFromEnv() Config {
	enabled := strings.EqualFold(strings.TrimSpace(os.Getenv("KOSCHEI_SENTINEL_OBSERVE_ENABLED")), "true") || strings.TrimSpace(os.Getenv("KOSCHEI_SENTINEL_OBSERVE_ENABLED")) == "1"
	return Config{
		Enabled: enabled,
		BaseURL: strings.TrimSpace(os.Getenv("KOSCHEI_SENTINEL_BASE_URL")),
		Token:   strings.TrimSpace(os.Getenv("KOSCHEI_SENTINEL_API_TOKEN")),
	}
}

func (c Config) Ready() bool {
	if !c.Enabled || c.BaseURL == "" || c.Token == "" {
		return false
	}
	_, err := outboundhttp.ValidateURL(strings.TrimRight(c.BaseURL, "/") + "/v1/opinions")
	return err == nil
}

type EvidenceItem struct {
	EvidenceID string         `json:"evidence_id"`
	Kind       string         `json:"kind"`
	Statement  string         `json:"statement"`
	Confidence string         `json:"confidence"`
	RuleIDs    []string       `json:"rule_ids"`
	Attributes map[string]any `json:"attributes"`
}

type SignedVerdict struct {
	Grade          string   `json:"grade"`
	Signature      string   `json:"signature"`
	TriggeredRules []string `json:"triggered_rules"`
	Summary        string   `json:"summary"`
}

type SecurityCase struct {
	SchemaVersion string         `json:"schema_version"`
	CaseID        string         `json:"case_id"`
	TargetRef     string         `json:"target_ref"`
	Network       string         `json:"network"`
	SignedVerdict SignedVerdict  `json:"signed_verdict"`
	Evidence      []EvidenceItem `json:"evidence"`
	Limitations   []string       `json:"limitations"`
}

type EvidenceClaim struct {
	Text        string   `json:"text"`
	EvidenceIDs []string `json:"evidence_ids"`
	Confidence  string   `json:"confidence"`
}

type Opinion struct {
	SchemaVersion      string          `json:"schema_version"`
	CaseID             string          `json:"case_id"`
	VerdictSignature   string          `json:"verdict_signature"`
	Authority          string          `json:"authority"`
	Assessment         string          `json:"assessment"`
	Claims             []EvidenceClaim `json:"claims"`
	Limitations        []string        `json:"limitations"`
	RecommendedActions []string        `json:"recommended_actions"`
	Engine             string          `json:"engine"`
	GeneratedAt        time.Time       `json:"generated_at"`
}

type CheckedOpinion struct {
	Opinion          Opinion  `json:"opinion"`
	PolicyViolations []string `json:"policy_violations"`
	Accepted         bool     `json:"accepted"`
}

type Observation struct {
	Mode       string         `json:"mode"`
	Authority  string         `json:"authority"`
	Accepted   bool           `json:"accepted"`
	Opinion    Opinion        `json:"opinion"`
	Violations []string       `json:"policy_violations"`
}

func Observe(ctx context.Context, client *http.Client, config Config, securityCase SecurityCase) (Observation, error) {
	if !config.Ready() {
		return Observation{}, errors.New("sentinel observe adapter is not configured")
	}
	body, err := json.Marshal(securityCase)
	if err != nil {
		return Observation{}, fmt.Errorf("encode Sentinel case: %w", err)
	}
	requestCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	endpoint := strings.TrimRight(config.BaseURL, "/") + "/v1/opinions"
	req, err := outboundhttp.NewRequest(requestCtx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return Observation{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+config.Token)
	req.Header.Set("User-Agent", "Koschei-Web3-Sentinel-Fabric/1.0")
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	resp, err := outboundhttp.Do(client, req)
	if err != nil {
		return Observation{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 16<<10))
		return Observation{}, fmt.Errorf("Sentinel returned HTTP %d", resp.StatusCode)
	}
	raw, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes+1))
	if err != nil || len(raw) > maxResponseBytes {
		return Observation{}, errors.New("Sentinel response is unavailable")
	}
	var checked CheckedOpinion
	if err := json.Unmarshal(raw, &checked); err != nil {
		return Observation{}, errors.New("Sentinel response is invalid")
	}
	if strings.TrimSpace(checked.Opinion.VerdictSignature) != strings.TrimSpace(securityCase.SignedVerdict.Signature) {
		return Observation{}, errors.New("Sentinel verdict signature mismatch")
	}
	if strings.TrimSpace(checked.Opinion.CaseID) != strings.TrimSpace(securityCase.CaseID) {
		return Observation{}, errors.New("Sentinel case identity mismatch")
	}
	return Observation{
		Mode:       "observe",
		Authority:  "commentary_only_arvis_verdict_final",
		Accepted:   checked.Accepted,
		Opinion:    checked.Opinion,
		Violations: checked.PolicyViolations,
	}, nil
}

// CaseFromARVISEnvelope projects only signed, evidence-backed ARVIS output into
// Sentinel's strict public contract. It does not invent missing evidence.
func CaseFromARVISEnvelope(envelope map[string]any) (SecurityCase, bool) {
	if envelope == nil {
		return SecurityCase{}, false
	}
	hasLiveEvidence, _ := envelope["has_live_evidence"].(bool)
	if !hasLiveEvidence {
		return SecurityCase{}, false
	}
	final, _ := envelope["final_verdict"].(map[string]any)
	if final == nil {
		return SecurityCase{}, false
	}
	signed, _ := final["signed"].(bool)
	grade := mapString(final, "grade")
	signature := mapString(final, "signature")
	if !signed || !validGrade(grade) || len(signature) < 8 {
		return SecurityCase{}, false
	}
	target := mapString(envelope, "target")
	network := mapString(envelope, "network")
	if target == "" || network == "" {
		return SecurityCase{}, false
	}

	evidence := evidenceFromArms(envelope["arms"])
	if len(evidence) == 0 {
		return SecurityCase{}, false
	}
	verdict := mapString(final, "verdict")
	if verdict == "" {
		verdict = "ARVIS signed deterministic verdict grade " + grade
	}
	limitations := stringSlice(envelope["limitations"])
	return SecurityCase{
		SchemaVersion: caseSchemaVersion,
		CaseID:        caseID(signature, target),
		TargetRef:     target,
		Network:       network,
		SignedVerdict: SignedVerdict{
			Grade:          grade,
			Signature:      signature,
			TriggeredRules: triggeredRuleIDs(final["triggered_rules"]),
			Summary:        trim(verdict, 4000),
		},
		Evidence:    evidence,
		Limitations: limitations,
	}, true
}

func evidenceFromArms(raw any) []EvidenceItem {
	arms, ok := raw.([]any)
	if !ok {
		return []EvidenceItem{}
	}
	out := make([]EvidenceItem, 0, len(arms))
	for _, rawArm := range arms {
		arm, ok := rawArm.(map[string]any)
		if !ok {
			continue
		}
		module := firstNonEmpty(mapString(arm, "module_id"), mapString(arm, "module"), "arvis")
		verified, _ := arm["evidence_verified"].(bool)
		confidence := "UNVERIFIED"
		if verified {
			confidence = "VERIFIED"
		}
		statements := stringSlice(arm["evidence"])
		for _, statement := range statements {
			statement = trim(statement, 4000)
			if statement == "" {
				continue
			}
			out = append(out, EvidenceItem{
				EvidenceID: evidenceID(module, statement),
				Kind:       trim(module, 64),
				Statement:  statement,
				Confidence: confidence,
				RuleIDs:    []string{},
				Attributes: map[string]any{"source": "arvis", "module": module},
			})
			if len(out) >= 512 {
				return out
			}
		}
	}
	return out
}

func triggeredRuleIDs(raw any) []string {
	values, ok := raw.([]any)
	if !ok {
		return []string{}
	}
	out := []string{}
	seen := map[string]bool{}
	for _, value := range values {
		var id string
		switch item := value.(type) {
		case string:
			id = strings.TrimSpace(item)
		case map[string]any:
			id = firstNonEmpty(mapString(item, "rule_id"), mapString(item, "id"))
		}
		if id != "" && !seen[id] {
			seen[id] = true
			out = append(out, trim(id, 128))
		}
		if len(out) >= 128 {
			break
		}
	}
	return out
}

func stringSlice(raw any) []string {
	values, ok := raw.([]any)
	if !ok {
		if values, ok := raw.([]string); ok {
			out := make([]string, 0, len(values))
			for _, value := range values {
				if value = strings.TrimSpace(value); value != "" {
					out = append(out, value)
				}
			return out
		}
		return []string{}
	}
	out := []string{}
	for _, rawValue := range values {
		if value, ok := rawValue.(string); ok {
			if value = strings.TrimSpace(value); value != "" {
				out = append(out, value)
			}
		}
	}
	return out
}

func evidenceID(module, statement string) string {
	sum := sha256.Sum256([]byte(module + "\n" + statement))
	return "arvis-" + hex.EncodeToString(sum[:16])
}

func caseID(signature, target string) string {
	sum := sha256.Sum256([]byte(signature + "\n" + target))
	return "arvis-" + hex.EncodeToString(sum[:16])
}

func mapString(values map[string]any, key string) string {
	if values == nil {
		return ""
	}
	value, _ := values[key].(string)
	return strings.TrimSpace(value)
}

func validGrade(value string) bool {
	switch strings.TrimSpace(value) {
	case "A", "B", "C", "D", "F", "-":
		return true
	default:
		return false
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			return value
		}
	}
	return ""
}

func trim(value string, limit int) string {
	value = strings.TrimSpace(value)
	if limit <= 0 || value == "" {
		return ""
	}
	runes := []rune(value)
	if len(runes) <= limit {
		return value
	}
	return string(runes[:limit])
}
