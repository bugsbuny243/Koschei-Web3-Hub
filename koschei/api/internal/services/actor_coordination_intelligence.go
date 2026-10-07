package services

import (
	"fmt"
	"sort"
	"strings"
)

const ActorCoordinationIntelligenceVersion = "koschei-actor-coordination-intelligence-v1"

type ActorCoordinationSignal struct {
	SignalID        string   `json:"signal_id"`
	Kind            string   `json:"kind"`
	EvidenceStatus  string   `json:"evidence_status"`
	SubjectWallet   string   `json:"subject_wallet"`
	RelatedWallets  []string `json:"related_wallets,omitempty"`
	TokenCount      int      `json:"token_count,omitempty"`
	EvidenceCount   int      `json:"evidence_count,omitempty"`
	Summary         string   `json:"summary"`
	GradeAuthority  bool     `json:"grade_authority"`
	IdentityClaim   bool     `json:"identity_claim"`
	WrongdoingClaim bool     `json:"wrongdoing_claim"`
}

type ActorCoordinationIntelligence struct {
	Version                 string                    `json:"version"`
	Network                 string                    `json:"network"`
	SubjectWallet           string                    `json:"subject_wallet"`
	Status                  string                    `json:"status"`
	Available               bool                      `json:"available"`
	Complete                bool                      `json:"complete"`
	SignalCount             int                       `json:"signal_count"`
	VerifiedSignalCount     int                       `json:"verified_signal_count"`
	ObservedSignalCount     int                       `json:"observed_signal_count"`
	Signals                 []ActorCoordinationSignal `json:"signals"`
	VerdictAuthority        bool                      `json:"verdict_authority"`
	SameOperatorClaim       bool                      `json:"same_operator_claim"`
	RealWorldIdentityClaim  bool                      `json:"real_world_identity_claim"`
	CriminalGroupClaim      bool                      `json:"criminal_group_claim"`
	MarketManipulationClaim bool                     `json:"market_manipulation_claim"`
	Limitations            []string                  `json:"limitations"`
}

// BuildActorCoordinationIntelligence projects retained actor evidence into a
// customer-facing coordination view. It deliberately does not create a second
// actor registry and does not change any deterministic grade. Shared funding,
// repeated roles and similar technical campaign descriptors are correlations;
// they are not proof that wallets share a controller, identity or criminal intent.
func BuildActorCoordinationIntelligence(dossier ActorDefenseDossier, funding PersistentFundingClusterReport, genome ActorCampaignGenome) ActorCoordinationIntelligence {
	subject := strings.TrimSpace(dossier.Wallet)
	if subject == "" {
		subject = strings.TrimSpace(funding.SubjectWallet)
	}
	network := normalizeRadarNetwork(dossier.Network)
	if strings.TrimSpace(dossier.Network) == "" && strings.TrimSpace(funding.Network) != "" {
		network = normalizeRadarNetwork(funding.Network)
	}
	out := ActorCoordinationIntelligence{
		Version:       ActorCoordinationIntelligenceVersion,
		Network:       network,
		SubjectWallet: subject,
		Status:        "no_coordination_pattern_observed",
		Signals:       []ActorCoordinationSignal{},
		Limitations: []string{
			"Coordination signals describe retained technical and on-chain relationships only; they do not prove common control, common identity, intent or wrongdoing.",
			"A shared funding source may be an exchange, service wallet, router or other infrastructure; it must not be treated as an operator identity claim without independent evidence.",
			"Market manipulation requires transaction and market-behavior evidence beyond wallet topology; this report does not issue a manipulation verdict.",
		},
	}
	if subject == "" {
		out.Status = "invalid_subject"
		return out
	}

	track := dossier.Track
	if track.CreatedTokenCount >= 2 {
		out.Signals = append(out.Signals, ActorCoordinationSignal{
			SignalID:       "repeat_creator_multi_token",
			Kind:           "repeat_creator_activity",
			EvidenceStatus: coordinationTrackStatus(track.Dossier, "creator_reuse_evidence_status"),
			SubjectWallet:  subject,
			TokenCount:     track.CreatedTokenCount,
			EvidenceCount:  track.CreatedTokenCount,
			Summary:        fmt.Sprintf("The same creator/deployer wallet is retained across %d token observations.", track.CreatedTokenCount),
		})
	}
	if track.DominantHolderTokenCount >= 2 {
		out.Signals = append(out.Signals, ActorCoordinationSignal{
			SignalID:       "repeat_dominant_holder_multi_token",
			Kind:           "repeat_dominant_holder_activity",
			EvidenceStatus: coordinationTrackStatus(track.Dossier, "holder_reuse_evidence_status"),
			SubjectWallet:  subject,
			TokenCount:     track.DominantHolderTokenCount,
			EvidenceCount:  track.DominantHolderTokenCount,
			Summary:        fmt.Sprintf("The same owner-resolved wallet is retained as a dominant holder across %d token observations.", track.DominantHolderTokenCount),
		})
	}
	if track.RelatedActorCount > 0 && actorDefenseStateRank(track.State) >= actorDefenseStateRank("correlated") {
		out.Signals = append(out.Signals, ActorCoordinationSignal{
			SignalID:       "cross_token_related_actor_recurrence",
			Kind:           "related_wallet_recurrence",
			EvidenceStatus: coordinationTrackStatus(track.Dossier, "related_actor_evidence_status"),
			SubjectWallet:  subject,
			EvidenceCount:  track.RelatedActorCount,
			Summary:        fmt.Sprintf("%d owner-resolved related wallet relationship(s) recur across the retained token surface.", track.RelatedActorCount),
		})
	}

	for index, source := range funding.Sources {
		if source.FundedActorCount < 2 {
			continue
		}
		wallets := coordinationClusterWallets(source.Members, subject, 25)
		status := coordinationFundingStatus(source.EvidenceStatus)
		out.Signals = append(out.Signals, ActorCoordinationSignal{
			SignalID:       fmt.Sprintf("shared_funder_cluster_%02d", index+1),
			Kind:           "shared_funding_cluster",
			EvidenceStatus: status,
			SubjectWallet:  subject,
			RelatedWallets: wallets,
			TokenCount:     source.CreatedTokenCount,
			EvidenceCount:  source.FundedActorCount,
			Summary:        fmt.Sprintf("Funding source %s is retained as funding %d actor wallet(s); %d created-token relation(s) are present in Koschei memory.", strings.TrimSpace(source.Wallet), source.FundedActorCount, source.CreatedTokenCount),
		})
		if source.LiquidityRemovalActors > 0 {
			out.Signals = append(out.Signals, ActorCoordinationSignal{
				SignalID:       fmt.Sprintf("shared_funder_liquidity_history_%02d", index+1),
				Kind:           "shared_funder_liquidity_history",
				EvidenceStatus: status,
				SubjectWallet:  subject,
				RelatedWallets: wallets,
				EvidenceCount:  source.LiquidityRemovalActors,
				Summary:        fmt.Sprintf("%d wallet(s) in the retained shared-funder cluster have observed or verified liquidity-removal evidence.", source.LiquidityRemovalActors),
			})
		}
	}

	if genome.Complete && strings.TrimSpace(genome.GenomeID) != "" {
		out.Signals = append(out.Signals, ActorCoordinationSignal{
			SignalID:       "verified_campaign_fingerprint",
			Kind:           "technical_campaign_fingerprint",
			EvidenceStatus: "verified",
			SubjectWallet:  subject,
			EvidenceCount:  genome.DescriptorCount,
			Summary:        fmt.Sprintf("Technical campaign fingerprint %s is backed by %d retained descriptor(s); similarity is a technical correlation, not an identity claim.", genome.GenomeID, genome.DescriptorCount),
		})
	}

	sort.SliceStable(out.Signals, func(i, j int) bool {
		return out.Signals[i].SignalID < out.Signals[j].SignalID
	})
	out.SignalCount = len(out.Signals)
	for i := range out.Signals {
		out.Signals[i].GradeAuthority = false
		out.Signals[i].IdentityClaim = false
		out.Signals[i].WrongdoingClaim = false
		switch out.Signals[i].EvidenceStatus {
		case "verified":
			out.VerifiedSignalCount++
		case "observed":
			out.ObservedSignalCount++
		}
	}
	out.Available = out.SignalCount > 0
	out.Complete = funding.Complete || out.SignalCount > 0
	if out.SignalCount > 0 {
		out.Status = "coordination_patterns_observed"
	}
	return out
}

func coordinationTrackStatus(dossier map[string]any, key string) string {
	if dossier != nil {
		raw := strings.ToLower(strings.TrimSpace(fmt.Sprint(dossier[key])))
		switch raw {
		case "verified", "verified_supported":
			return "verified"
		case "observed", "observed_supported":
			return "observed"
		}
	}
	return "observed"
}

func coordinationFundingStatus(status string) string {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "verified", "verified_supported":
		return "verified"
	default:
		return "observed"
	}
}

func coordinationClusterWallets(members []PersistentFundingClusterMember, subject string, limit int) []string {
	if limit <= 0 {
		limit = 25
	}
	seen := map[string]struct{}{}
	out := []string{}
	for _, member := range members {
		wallet := strings.TrimSpace(member.Wallet)
		if wallet == "" || wallet == strings.TrimSpace(subject) {
			continue
		}
		if _, ok := seen[wallet]; ok {
			continue
		}
		seen[wallet] = struct{}{}
		out = append(out, wallet)
		if len(out) >= limit {
			break
		}
	}
	sort.Strings(out)
	return out
}
