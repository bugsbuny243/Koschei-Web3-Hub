package services

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"sort"
	"strings"
)

const (
	GlobalCampaignResponseProductionReadinessSchemaVersion = "koschei.response-production-readiness.v1"
	GlobalCampaignResponseProductionReadinessBlocked       = "BLOCKED"
	GlobalCampaignResponseProductionReadinessReady         = "READY"

	GlobalCampaignResponseProductionBlockerForwarderMissing      = "concrete_production_forwarder_missing"
	GlobalCampaignResponseProductionBlockerDeploymentUnpinned    = "deployed_revision_unpinned"
	GlobalCampaignResponseProductionBlockerForwarderIdentity     = "forwarder_identity_missing"
	GlobalCampaignResponseProductionBlockerForwarderArtifact     = "forwarder_artifact_unverified"
	GlobalCampaignResponseProductionBlockerProductionIdentity    = "production_identity_missing"
	GlobalCampaignResponseProductionBlockerEffectCollector       = "independent_effect_collector_missing"
	GlobalCampaignResponseProductionBlockerAuthorizationContract = "response_authorization_contract_mismatch"
	GlobalCampaignResponseProductionBlockerEffectProofContract   = "response_effect_proof_contract_mismatch"
)

// This stays false until a concrete production forwarder implementation is
// added, reviewed and bound to exact deployment/runtime evidence. The current
// repository intentionally has no autonomous chain-submission authority here.
const globalCampaignResponseConcreteProductionForwarderLinked = false

var ErrGlobalCampaignResponseProductionReadinessInvalid = errors.New("global campaign response production readiness is invalid")

type GlobalCampaignResponseProductionReadinessEvidence struct {
	DeploymentRef            string `json:"deployment_ref"`
	DeployedRevision         string `json:"deployed_revision"`
	ForwarderRef             string `json:"forwarder_ref"`
	ForwarderArtifactSHA256  string `json:"forwarder_artifact_sha256"`
	ProductionIdentityRef    string `json:"production_identity_ref"`
	EffectCollectorProducer  string `json:"effect_collector_producer"`
	EffectCollectorPublicKey string `json:"effect_collector_public_key"`
	AuthorizationSchema      string `json:"authorization_schema"`
	EffectProofSchema        string `json:"effect_proof_schema"`
}

type GlobalCampaignResponseProductionReadiness struct {
	SchemaVersion              string                                            `json:"schema_version"`
	State                      string                                            `json:"state"`
	Evidence                   GlobalCampaignResponseProductionReadinessEvidence `json:"evidence"`
	ConcreteForwarderLinked    bool                                              `json:"concrete_forwarder_linked"`
	ProductionContainmentReady bool                                              `json:"production_containment_ready"`
	ProductionClaimAllowed     bool                                              `json:"production_claim_allowed"`
	Blockers                   []string                                          `json:"blockers"`
	ReadinessHashSHA256        string                                            `json:"readiness_hash_sha256"`
}

func EvaluateGlobalCampaignResponseProductionReadiness(evidence GlobalCampaignResponseProductionReadinessEvidence) GlobalCampaignResponseProductionReadiness {
	evidence = normalizeGlobalCampaignResponseProductionReadinessEvidence(evidence)
	blockers := make([]string, 0, 8)
	if !globalCampaignResponseConcreteProductionForwarderLinked {
		blockers = append(blockers, GlobalCampaignResponseProductionBlockerForwarderMissing)
	}
	if evidence.DeploymentRef == "" || !validGitRevisionForResponseReadiness(evidence.DeployedRevision) {
		blockers = append(blockers, GlobalCampaignResponseProductionBlockerDeploymentUnpinned)
	}
	if evidence.ForwarderRef == "" {
		blockers = append(blockers, GlobalCampaignResponseProductionBlockerForwarderIdentity)
	}
	if !validResponseDigest(evidence.ForwarderArtifactSHA256) {
		blockers = append(blockers, GlobalCampaignResponseProductionBlockerForwarderArtifact)
	}
	if evidence.ProductionIdentityRef == "" {
		blockers = append(blockers, GlobalCampaignResponseProductionBlockerProductionIdentity)
	}
	if evidence.EffectCollectorProducer == "" || !validResponseReadinessEd25519PublicKey(evidence.EffectCollectorPublicKey) ||
		strings.EqualFold(evidence.EffectCollectorProducer, evidence.ProductionIdentityRef) {
		blockers = append(blockers, GlobalCampaignResponseProductionBlockerEffectCollector)
	}
	if evidence.AuthorizationSchema != GlobalCampaignResponseAuthorizationSchemaVersion {
		blockers = append(blockers, GlobalCampaignResponseProductionBlockerAuthorizationContract)
	}
	if evidence.EffectProofSchema != GlobalCampaignEffectProofSchemaVersion {
		blockers = append(blockers, GlobalCampaignResponseProductionBlockerEffectProofContract)
	}
	sort.Strings(blockers)

	ready := len(blockers) == 0
	state := GlobalCampaignResponseProductionReadinessBlocked
	if ready {
		state = GlobalCampaignResponseProductionReadinessReady
	}
	status := GlobalCampaignResponseProductionReadiness{
		SchemaVersion:              GlobalCampaignResponseProductionReadinessSchemaVersion,
		State:                      state,
		Evidence:                   evidence,
		ConcreteForwarderLinked:    globalCampaignResponseConcreteProductionForwarderLinked,
		ProductionContainmentReady: ready,
		ProductionClaimAllowed:     ready,
		Blockers:                   blockers,
	}
	status.ReadinessHashSHA256 = hashGlobalCampaignResponseProductionReadiness(status)
	return status
}

func ValidateGlobalCampaignResponseProductionReadiness(status GlobalCampaignResponseProductionReadiness) error {
	if status.SchemaVersion != GlobalCampaignResponseProductionReadinessSchemaVersion || !validResponseDigest(status.ReadinessHashSHA256) {
		return ErrGlobalCampaignResponseProductionReadinessInvalid
	}
	recomputed := EvaluateGlobalCampaignResponseProductionReadiness(status.Evidence)
	if recomputed.State != status.State ||
		recomputed.ConcreteForwarderLinked != status.ConcreteForwarderLinked ||
		recomputed.ProductionContainmentReady != status.ProductionContainmentReady ||
		recomputed.ProductionClaimAllowed != status.ProductionClaimAllowed ||
		!equalGlobalCampaignResponseReadinessStrings(recomputed.Blockers, status.Blockers) ||
		recomputed.ReadinessHashSHA256 != status.ReadinessHashSHA256 {
		return ErrGlobalCampaignResponseProductionReadinessInvalid
	}
	if !globalCampaignResponseConcreteProductionForwarderLinked && (status.State == GlobalCampaignResponseProductionReadinessReady || status.ProductionContainmentReady || status.ProductionClaimAllowed) {
		return ErrGlobalCampaignResponseProductionReadinessInvalid
	}
	return nil
}

func normalizeGlobalCampaignResponseProductionReadinessEvidence(evidence GlobalCampaignResponseProductionReadinessEvidence) GlobalCampaignResponseProductionReadinessEvidence {
	evidence.DeploymentRef = strings.TrimSpace(evidence.DeploymentRef)
	evidence.DeployedRevision = strings.ToLower(strings.TrimSpace(evidence.DeployedRevision))
	evidence.ForwarderRef = strings.TrimSpace(evidence.ForwarderRef)
	evidence.ForwarderArtifactSHA256 = normalizeResponseDigest(evidence.ForwarderArtifactSHA256)
	evidence.ProductionIdentityRef = strings.TrimSpace(evidence.ProductionIdentityRef)
	evidence.EffectCollectorProducer = strings.TrimSpace(evidence.EffectCollectorProducer)
	evidence.EffectCollectorPublicKey = strings.TrimSpace(evidence.EffectCollectorPublicKey)
	evidence.AuthorizationSchema = strings.TrimSpace(evidence.AuthorizationSchema)
	evidence.EffectProofSchema = strings.TrimSpace(evidence.EffectProofSchema)
	return evidence
}

func validGitRevisionForResponseReadiness(value string) bool {
	value = strings.TrimSpace(value)
	if len(value) != 40 && len(value) != 64 {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}

func validResponseReadinessEd25519PublicKey(value string) bool {
	decoded, err := base64.RawURLEncoding.DecodeString(strings.TrimSpace(value))
	return err == nil && len(decoded) == ed25519.PublicKeySize && base64.RawURLEncoding.EncodeToString(decoded) == strings.TrimSpace(value)
}

func hashGlobalCampaignResponseProductionReadiness(status GlobalCampaignResponseProductionReadiness) string {
	status.ReadinessHashSHA256 = ""
	payload, err := json.Marshal(status)
	if err != nil {
		return ""
	}
	digest := sha256.Sum256(payload)
	return "sha256:" + hex.EncodeToString(digest[:])
}

func equalGlobalCampaignResponseReadinessStrings(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}
