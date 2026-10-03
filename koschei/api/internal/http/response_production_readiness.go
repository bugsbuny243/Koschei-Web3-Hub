package http

import (
	"os"
	"strings"

	"koschei/api/internal/services"
)

const (
	responseDeploymentRefEnv            = "KOSCHEI_RESPONSE_DEPLOYMENT_REF"
	responseForwarderRefEnv             = "KOSCHEI_RESPONSE_FORWARDER_REF"
	responseForwarderArtifactSHAEnv     = "KOSCHEI_RESPONSE_FORWARDER_ARTIFACT_SHA256"
	responseProductionIdentityRefEnv    = "KOSCHEI_RESPONSE_PRODUCTION_IDENTITY_REF"
	responseEffectCollectorProducerEnv  = "KOSCHEI_RESPONSE_EFFECT_COLLECTOR_PRODUCER"
	responseEffectCollectorPublicKeyEnv = "KOSCHEI_RESPONSE_EFFECT_COLLECTOR_PUBLIC_KEY"
)

func currentResponseProductionReadiness() services.GlobalCampaignResponseProductionReadiness {
	return responseProductionReadinessFrom(os.Getenv)
}

func responseProductionReadinessFrom(getenv func(string) string) services.GlobalCampaignResponseProductionReadiness {
	if getenv == nil {
		getenv = func(string) string { return "" }
	}
	evidence := services.GlobalCampaignResponseProductionReadinessEvidence{
		DeploymentRef:            strings.TrimSpace(getenv(responseDeploymentRefEnv)),
		DeployedRevision:         deploymentRevisionFrom(getenv),
		ForwarderRef:             strings.TrimSpace(getenv(responseForwarderRefEnv)),
		ForwarderArtifactSHA256:  strings.TrimSpace(getenv(responseForwarderArtifactSHAEnv)),
		ProductionIdentityRef:    strings.TrimSpace(getenv(responseProductionIdentityRefEnv)),
		EffectCollectorProducer:  strings.TrimSpace(getenv(responseEffectCollectorProducerEnv)),
		EffectCollectorPublicKey: strings.TrimSpace(getenv(responseEffectCollectorPublicKeyEnv)),
		AuthorizationSchema:      services.GlobalCampaignResponseAuthorizationSchemaVersion,
		EffectProofSchema:        services.GlobalCampaignEffectProofSchemaVersion,
	}
	return services.EvaluateGlobalCampaignResponseProductionReadiness(evidence)
}

func ownerResponseProductionReadinessTelemetry() map[string]any {
	status := currentResponseProductionReadiness()
	return map[string]any{
		"schema_version":               status.SchemaVersion,
		"state":                        status.State,
		"verification":                 "runtime_configuration_plus_repository_guard",
		"production_containment_ready": status.ProductionContainmentReady,
		"production_claim_allowed":     status.ProductionClaimAllowed,
		"concrete_forwarder_linked":    status.ConcreteForwarderLinked,
		"blocker_count":                len(status.Blockers),
		"blockers":                     status.Blockers,
		"readiness_hash_sha256":        status.ReadinessHashSHA256,
		"deployment_ref":               status.Evidence.DeploymentRef,
		"deployed_revision":            status.Evidence.DeployedRevision,
		"authorization_schema":         status.Evidence.AuthorizationSchema,
		"effect_proof_schema":          status.Evidence.EffectProofSchema,
	}
}
