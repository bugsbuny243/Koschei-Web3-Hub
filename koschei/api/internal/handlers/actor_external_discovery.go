package handlers

import (
	"context"
	"strings"

	"koschei/api/internal/services"
)

type actorExternalDiscoveryRun struct {
	Status              string                          `json:"status"`
	Discovery           services.SolscanActorDiscovery `json:"discovery"`
	EvidenceProduced    int                             `json:"evidence_produced"`
	EvidencePersisted   int                             `json:"evidence_persisted"`
	PersistenceFailures int                             `json:"persistence_failures"`
	Limitations         []string                        `json:"limitations"`
}

func (h *Handler) collectActorExternalDiscovery(ctx context.Context, store *services.ActorDefenseStore, wallet, network string) actorExternalDiscoveryRun {
	wallet = strings.TrimSpace(wallet)
	out := actorExternalDiscoveryRun{
		Status: "not_requested",
		Discovery: services.SolscanActorDiscovery{
			Status: "not_requested", Provider: "solscan_pro_api_v2", Wallet: wallet,
			TransactionCandidates: []services.SolscanAccountTransaction{},
			TokenAccounts: []services.SolscanTokenAccountObservation{},
			EndpointStatus: map[string]string{}, Limitations: []string{},
		},
		Limitations: []string{},
	}
	if wallet == "" {
		out.Status = "wallet_required"
		out.Limitations = append(out.Limitations, "Solscan actor discovery için wallet hedefi çözümlenemedi.")
		return out
	}

	out.Discovery = services.FetchSolscanActorDiscovery(ctx, wallet, 40)
	out.Status = out.Discovery.Status
	out.Limitations = append(out.Limitations, out.Discovery.Limitations...)
	if !out.Discovery.Available {
		return out
	}

	evidence := services.SolscanActorDiscoveryEvidence(out.Discovery, network)
	out.EvidenceProduced = len(evidence)
	if len(evidence) == 0 {
		if out.Status == "complete" {
			out.Status = "complete_no_persistable_relations"
		}
		return out
	}
	if store == nil {
		out.Status = "persistence_unavailable"
		out.Limitations = append(out.Limitations, "Solscan discovery tamamlandı ancak actor evidence store kullanılamıyor.")
		return out
	}

	for _, item := range evidence {
		if err := store.UpsertEvidence(ctx, item); err != nil {
			out.PersistenceFailures++
			continue
		}
		out.EvidencePersisted++
	}
	if out.PersistenceFailures > 0 {
		out.Status = "partial_persistence"
		out.Limitations = append(out.Limitations, "Bazı Solscan discovery ilişkileri kalıcı actor index'e yazılamadı.")
	} else if out.Status == "complete" {
		out.Status = "complete_persisted"
	} else if out.Status == "partial" {
		out.Status = "partial_persisted"
	}
	return out
}
