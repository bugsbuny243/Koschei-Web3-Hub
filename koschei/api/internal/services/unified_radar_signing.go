package services

import (
	"crypto/ed25519"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
)

const (
	UnifiedVerdictSignatureDomainV1    = "koschei.arvis-verdict/v1"
	UnifiedVerdictSignatureAlgorithmV1 = "ed25519"
	unifiedVerdictSigningKeyIDEnv      = "KOSCHEI_VERDICT_SIGNING_KEY_ID"
	unifiedVerdictSigningPrivateKeyEnv = "KOSCHEI_VERDICT_SIGNING_PRIVATE_KEY"
)

type unifiedVerdictRuleBindingV1 struct {
	RuleID         string   `json:"rule_id"`
	EvidenceStatus string   `json:"evidence_status"`
	Title          string   `json:"title"`
	Tier           string   `json:"tier"`
	GradeEffect    string   `json:"grade_effect"`
	GradeCap       string   `json:"grade_cap"`
	Count          int      `json:"count"`
	Summary        string   `json:"summary"`
	EvidenceKeys   []string `json:"evidence_keys"`
	Signatures     []string `json:"signatures"`
}

type unifiedVerdictSigningPayloadV1 struct {
	SchemaVersion  string                        `json:"schema_version"`
	Target         string                        `json:"target"`
	Network        string                        `json:"network"`
	Grade          string                        `json:"grade"`
	Verdict        string                        `json:"verdict"`
	Evidence       []string                      `json:"evidence"`
	RuleVersion    string                        `json:"rule_version"`
	TriggeredRules []unifiedVerdictRuleBindingV1 `json:"triggered_rules"`
	WatchFlags     []unifiedVerdictRuleBindingV1 `json:"watch_flags"`
	DecisionPath   []string                      `json:"decision_path"`
	CreatedAt      string                        `json:"created_at"`
}

type unifiedVerdictAuthenticationV1 struct {
	Algorithm   string
	KeyID       string
	PayloadHash string
	Signature   string
}

func authenticateUnifiedRadarVerdictV1(verdict UnifiedRadarVerdict) (unifiedVerdictAuthenticationV1, bool, error) {
	keyID, privateKey, configured, err := unifiedVerdictSignerFromEnv()
	if err != nil || !configured {
		return unifiedVerdictAuthenticationV1{}, configured, err
	}
	payload, err := unifiedVerdictSigningBytesV1(verdict)
	if err != nil {
		return unifiedVerdictAuthenticationV1{}, true, err
	}
	sum := sha256.Sum256(payload)
	return unifiedVerdictAuthenticationV1{
		Algorithm:   UnifiedVerdictSignatureAlgorithmV1,
		KeyID:       keyID,
		PayloadHash: "sha256:" + hex.EncodeToString(sum[:]),
		Signature:   base64.RawURLEncoding.EncodeToString(ed25519.Sign(privateKey, payload)),
	}, true, nil
}

func unifiedVerdictSigningBytesV1(verdict UnifiedRadarVerdict) ([]byte, error) {
	target := strings.TrimSpace(verdict.Target)
	network := strings.TrimSpace(verdict.Network)
	if target == "" || network == "" {
		return nil, errors.New("authenticated verdict requires target and network")
	}
	if verdict.GeneratedAt.IsZero() {
		return nil, errors.New("authenticated verdict requires created_at")
	}
	evidence := unifiedVerdictContractEvidence(verdict)
	if len(evidence) == 0 {
		return nil, errors.New("authenticated verdict requires evidence")
	}
	payload := unifiedVerdictSigningPayloadV1{
		SchemaVersion:  UnifiedVerdictSignatureDomainV1,
		Target:         target,
		Network:        network,
		Grade:          normalizeUnifiedContractGrade(verdict.Grade),
		Verdict:        strings.TrimSpace(verdict.Verdict),
		Evidence:       trimmedUnifiedVerdictStrings(evidence),
		RuleVersion:    strings.TrimSpace(verdict.RulesetVersion),
		TriggeredRules: unifiedVerdictRuleBindingsV1(verdict.TriggeredRules),
		WatchFlags:     unifiedVerdictRuleBindingsV1(verdict.WatchFlags),
		DecisionPath:   trimmedUnifiedVerdictStrings(nonNilStrings(verdict.DecisionPath)),
		CreatedAt:      verdict.GeneratedAt.UTC().Format("2006-01-02T15:04:05.999999999Z07:00"),
	}
	if payload.Verdict == "" || payload.RuleVersion == "" {
		return nil, errors.New("authenticated verdict requires verdict and rule version")
	}
	return json.Marshal(payload)
}

func unifiedVerdictRuleBindingsV1(hits []ActorDefenseRuleHit) []unifiedVerdictRuleBindingV1 {
	out := make([]unifiedVerdictRuleBindingV1, 0, len(hits))
	for _, hit := range hits {
		out = append(out, unifiedVerdictRuleBindingV1{
			RuleID:         strings.TrimSpace(hit.RuleID),
			EvidenceStatus: strings.TrimSpace(hit.EvidenceStatus),
			Title:          strings.TrimSpace(hit.Title),
			Tier:           strings.TrimSpace(hit.Tier),
			GradeEffect:    strings.TrimSpace(hit.GradeEffect),
			GradeCap:       strings.TrimSpace(hit.GradeCap),
			Count:          hit.Count,
			Summary:        strings.TrimSpace(hit.Summary),
			EvidenceKeys:   trimmedUnifiedVerdictStrings(hit.EvidenceKeys),
			Signatures:     trimmedUnifiedVerdictStrings(hit.Signatures),
		})
	}
	return out
}

func trimmedUnifiedVerdictStrings(values []string) []string {
	if values == nil {
		return []string{}
	}
	out := make([]string, len(values))
	for i, value := range values {
		out[i] = strings.TrimSpace(value)
	}
	return out
}

func unifiedVerdictSignerFromEnv() (string, ed25519.PrivateKey, bool, error) {
	keyID := strings.TrimSpace(os.Getenv(unifiedVerdictSigningKeyIDEnv))
	rawKey := strings.TrimSpace(os.Getenv(unifiedVerdictSigningPrivateKeyEnv))
	if keyID == "" && rawKey == "" {
		return "", nil, false, nil
	}
	if keyID == "" || rawKey == "" {
		return "", nil, true, fmt.Errorf("%s and %s must be configured together", unifiedVerdictSigningKeyIDEnv, unifiedVerdictSigningPrivateKeyEnv)
	}
	privateKey, err := parseUnifiedVerdictPrivateKey(rawKey)
	if err != nil {
		return "", nil, true, err
	}
	return keyID, privateKey, true, nil
}

func parseUnifiedVerdictPrivateKey(raw string) (ed25519.PrivateKey, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, errors.New("verdict signing private key is empty")
	}
	decoders := []func(string) ([]byte, error){
		base64.RawURLEncoding.DecodeString,
		base64.URLEncoding.DecodeString,
		base64.RawStdEncoding.DecodeString,
		base64.StdEncoding.DecodeString,
		hex.DecodeString,
	}
	for _, decode := range decoders {
		decoded, err := decode(raw)
		if err != nil {
			continue
		}
		switch len(decoded) {
		case ed25519.SeedSize:
			return ed25519.NewKeyFromSeed(decoded), nil
		case ed25519.PrivateKeySize:
			privateKey := ed25519.PrivateKey(append([]byte(nil), decoded...))
			derived := ed25519.NewKeyFromSeed(privateKey[:ed25519.SeedSize])
			if subtle.ConstantTimeCompare(privateKey, derived) != 1 {
				return nil, errors.New("verdict signing private key is not canonical seed-derived Ed25519 material")
			}
			return privateKey, nil
		}
	}
	return nil, errors.New("verdict signing private key must decode to a 32-byte Ed25519 seed or canonical 64-byte private key")
}
