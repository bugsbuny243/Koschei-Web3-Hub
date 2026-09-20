package services

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
)

const (
	SecurityRadarVerdictSignatureDomainV1 = "koschei.arvis-arm-verdict/v1"
	securityRadarVerdictDigestPrefix      = "koschei-radar:"
)

type securityRadarVerdictSigningPayloadV1 struct {
	SchemaVersion    string   `json:"schema_version"`
	ModuleID         string   `json:"module_id"`
	Target           string   `json:"target"`
	Network          string   `json:"network"`
	Grade            string   `json:"grade"`
	RiskIndex        int      `json:"risk_index"`
	RiskLevel        string   `json:"risk_level"`
	Verdict          string   `json:"verdict"`
	Recommendation   string   `json:"recommendation"`
	Evidence         []string `json:"evidence"`
	RuleVersion      string   `json:"rule_version"`
	EvidenceVerified bool     `json:"evidence_verified"`
}

func finalizeSecurityRadarVerdictAuthentication(verdict SecurityRadarVerdict) SecurityRadarVerdict {
	verdict.ModuleID = strings.TrimSpace(verdict.ModuleID)
	verdict.Target = strings.TrimSpace(verdict.Target)
	verdict.Network = normalizeRadarNetwork(verdict.Network)
	verdict.RuleVersion = strings.TrimSpace(verdict.RuleVersion)
	verdict.EvidenceVerified = verdict.EvidenceVerified || securityRadarSignalsHaveVerifiedEvidence(verdict.Signals)
	verdict.Digest = ""

	clearSecurityRadarVerdictAuthentication(&verdict)

	payload, err := securityRadarVerdictSigningBytesV1(verdict)
	if err != nil {
		return verdict
	}
	sum := sha256.Sum256(payload)
	verdict.Digest = securityRadarVerdictDigestPrefix + hex.EncodeToString(sum[:])

	keyID, privateKey, configured, err := unifiedVerdictSignerFromEnv()
	if err != nil || !configured {
		return verdict
	}
	verdict.Signed = true
	verdict.SignatureAlgorithm = UnifiedVerdictSignatureAlgorithmV1
	verdict.KeyID = keyID
	verdict.PayloadHash = "sha256:" + hex.EncodeToString(sum[:])
	verdict.Signature = base64.RawURLEncoding.EncodeToString(ed25519.Sign(privateKey, payload))
	return verdict
}

func clearSecurityRadarVerdictAuthentication(verdict *SecurityRadarVerdict) {
	if verdict == nil {
		return
	}
	verdict.Signed = false
	verdict.Signature = ""
	verdict.SignatureAlgorithm = ""
	verdict.KeyID = ""
	verdict.PayloadHash = ""
}

func securityRadarVerdictSigningBytesV1(verdict SecurityRadarVerdict) ([]byte, error) {
	moduleID := strings.TrimSpace(verdict.ModuleID)
	target := strings.TrimSpace(verdict.Target)
	network := strings.TrimSpace(verdict.Network)
	ruleVersion := strings.TrimSpace(verdict.RuleVersion)
	if moduleID == "" || target == "" || network == "" || ruleVersion == "" {
		return nil, errors.New("authenticated radar verdict requires module_id, target, network and rule_version")
	}
	payload := securityRadarVerdictSigningPayloadV1{
		SchemaVersion:    SecurityRadarVerdictSignatureDomainV1,
		ModuleID:         moduleID,
		Target:           target,
		Network:          network,
		Grade:            strings.TrimSpace(verdict.Grade),
		RiskIndex:        verdict.RiskIndex,
		RiskLevel:        strings.TrimSpace(verdict.RiskLevel),
		Verdict:          strings.TrimSpace(verdict.Verdict),
		Recommendation:   strings.TrimSpace(verdict.Recommendation),
		Evidence:         trimmedUnifiedVerdictStrings(nonNilEvidence(verdict.Evidence)),
		RuleVersion:      ruleVersion,
		EvidenceVerified: verdict.EvidenceVerified,
	}
	if payload.Verdict == "" {
		return nil, errors.New("authenticated radar verdict requires verdict")
	}
	return json.Marshal(payload)
}
