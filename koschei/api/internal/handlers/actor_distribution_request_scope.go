package handlers

import (
	"context"
	"strings"
	"time"

	"koschei/api/internal/services"
)

// collectRequestScopeCanonicalActorDistribution runs the existing bounded,
// mint-specific recipient investigator without requiring persistent actor
// storage. It is intentionally gated on a VERIFIED canonical creator->mint
// relation so an observed-only attribution cannot be upgraded through derived
// recipient evidence.
func (h *Handler) collectRequestScopeCanonicalActorDistribution(ctx context.Context, relation actorCreatorRelationRun, network string) (actorDistributionIntegrationRun, []services.ActorDefenseEvidenceRecord) {
	out := newActorDistributionIntegrationRun(relation.Target.CreatorWallet, relation.Target.Mint)
	out.Target = relation.Target
	evidence := []services.ActorDefenseEvidenceRecord{}

	if relation.Persistence != "request_scope" {
		out.Status = "creator_mint_relation_unavailable"
		out.Limitations = append(out.Limitations, "Request-scope creator → mint ilişkisi bulunmadığı için stateless distribution araştırması başlatılmadı.")
		return out, evidence
	}
	if !strings.EqualFold(strings.TrimSpace(relation.Target.VerificationStatus), "verified") {
		out.Status = "creator_mint_relation_observed_only"
		out.Limitations = append(out.Limitations, "Creator → mint ilişkisi VERIFIED olmadığı için derived recipient evidence üretilmedi.")
		return out, evidence
	}

	timeout := time.Duration(actorDefenseEnvInt("ACTOR_RECIPIENT_TIMEOUT_SECONDS", 150, 30, 240)) * time.Second
	distributionCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	out.Report = services.InvestigateActorInitialRecipients(
		distributionCtx,
		creatorIntelRPCURL(),
		relation.Target.CreatorWallet,
		relation.Target.Mint,
		relation.Target.CreationSignature,
		services.ActorInitialRecipientOptions{
			MaxRecipients:        actorDefenseEnvInt("ACTOR_RECIPIENT_LIMIT", 20, 1, 20),
			SignaturePageSize:    actorDefenseEnvInt("ACTOR_RECIPIENT_SIGNATURE_PAGE_SIZE", 250, 50, 1000),
			MaxPagesPerTokenATA:  actorDefenseEnvInt("ACTOR_RECIPIENT_MAX_PAGES_PER_ATA", 8, 1, 20),
			MaxTransactionsParse: actorDefenseEnvInt("ACTOR_RECIPIENT_TRANSACTION_LIMIT", 160, 10, 500),
		},
	)
	out.Status = out.Report.Status
	out.Limitations = append(out.Limitations, out.Report.Limitations...)
	evidence = requestScopeActorDistributionEvidence(out.Report, network)
	out.EvidenceProduced = len(evidence)
	return out, evidence
}

func requestScopeActorDistributionEvidence(report services.ActorInitialRecipientReport, network string) []services.ActorDefenseEvidenceRecord {
	evidence := services.ActorInitialRecipientEvidence(report, network)
	for index := range evidence {
		if evidence[index].Metadata == nil {
			evidence[index].Metadata = map[string]any{}
		}
		evidence[index].Metadata["request_scope_actor_evidence"] = true
		evidence[index].Metadata["request_scope_distribution"] = true
		evidence[index].Metadata["persistent_actor_index"] = false
		evidence[index].Metadata["identity_or_wrongdoing_claim"] = false
	}
	return evidence
}

func applyRequestScopeActorDistributionEvidence(dossier services.ActorDefenseDossier, evidence []services.ActorDefenseEvidenceRecord) services.ActorDefenseDossier {
	if dossier.Evidence == nil {
		dossier.Evidence = []services.ActorDefenseEvidenceRecord{}
	}
	seen := map[string]bool{}
	for _, item := range dossier.Evidence {
		if key := requestScopeActorEvidenceIdentity(item); key != "" {
			seen[key] = true
		}
	}
	for _, item := range evidence {
		key := requestScopeActorEvidenceIdentity(item)
		if key == "" || seen[key] {
			continue
		}
		dossier.Evidence = append(dossier.Evidence, item)
		seen[key] = true
	}
	return dossier
}
