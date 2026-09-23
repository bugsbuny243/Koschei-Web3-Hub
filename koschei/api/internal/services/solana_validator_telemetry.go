package services

import (
	"context"
	"fmt"
	"strings"
	"time"

	"koschei/api/internal/networktarget"
)

type SolanaVoteAccountTelemetry struct {
	VotePubkey     string                                    `json:"vote_pubkey"`
	NodePubkey     string                                    `json:"node_pubkey"`
	ActivatedStake uint64                                    `json:"activated_stake"`
	Commission     int                                       `json:"commission"`
	LastVote       uint64                                    `json:"last_vote"`
	RootSlot       uint64                                    `json:"root_slot"`
	Delinquent     bool                                      `json:"delinquent"`
	Observation    networktarget.NetworkTelemetryObservation `json:"observation"`
}

type SolanaValidatorTelemetryReport struct {
	SchemaVersion       string                       `json:"schema_version"`
	Network             string                       `json:"network"`
	CurrentCount        int                          `json:"current_count"`
	DelinquentCount     int                          `json:"delinquent_count"`
	TotalActivatedStake uint64                       `json:"total_activated_stake"`
	Validators          []SolanaVoteAccountTelemetry `json:"validators"`
	Source              string                       `json:"source"`
	ObservedAt          time.Time                    `json:"observed_at"`
	AnalysisPerformed   bool                         `json:"analysis_performed"`
	LiveAvailability    string                       `json:"live_availability"`
}

type solanaVoteAccountRPCItem struct {
	VotePubkey       string `json:"votePubkey"`
	NodePubkey       string `json:"nodePubkey"`
	ActivatedStake   uint64 `json:"activatedStake"`
	Commission       int    `json:"commission"`
	LastVote         uint64 `json:"lastVote"`
	RootSlot         uint64 `json:"rootSlot"`
	EpochVoteAccount bool   `json:"epochVoteAccount"`
}

type solanaVoteAccountsRPCResult struct {
	Current    []solanaVoteAccountRPCItem `json:"current"`
	Delinquent []solanaVoteAccountRPCItem `json:"delinquent"`
}

// ProbeSolanaValidatorTelemetry observes the current vote-account set through
// the existing governed Solana RPC client, so background budget, provider
// cooldown and HTTP failover policies remain in force.
func ProbeSolanaValidatorTelemetry(ctx context.Context, rpcURL string, observedAt time.Time) (SolanaValidatorTelemetryReport, error) {
	rpcURL = strings.TrimSpace(rpcURL)
	if rpcURL == "" {
		return SolanaValidatorTelemetryReport{}, fmt.Errorf("solana rpc url is empty")
	}
	if observedAt.IsZero() {
		return SolanaValidatorTelemetryReport{}, fmt.Errorf("solana_validator_telemetry_observed_at_required")
	}

	result, err := solanaRPCDo[solanaVoteAccountsRPCResult](ctx, rpcURL, "getVoteAccounts", []any{map[string]any{"commitment": "confirmed", "keepUnstakedDelinquents": true}})
	if err != nil {
		return SolanaValidatorTelemetryReport{}, err
	}

	total, err := totalSolanaActivatedStake(result.Current, result.Delinquent)
	if err != nil {
		return SolanaValidatorTelemetryReport{}, err
	}

	validators := make([]SolanaVoteAccountTelemetry, 0, len(result.Current)+len(result.Delinquent))
	appendItems := func(items []solanaVoteAccountRPCItem, delinquent bool) error {
		for _, item := range items {
			if strings.TrimSpace(item.VotePubkey) == "" || strings.TrimSpace(item.NodePubkey) == "" || item.Commission < 0 || item.Commission > 100 {
				return fmt.Errorf("solana_validator_telemetry_item_invalid")
			}
			var share *float64
			if total > 0 {
				value := (float64(item.ActivatedStake) / float64(total)) * 100
				share = &value
			}
			observation, err := networktarget.NormalizeNetworkTelemetry(networktarget.NetworkTelemetryInput{
				NetworkID:      "solana-mainnet",
				SubjectKind:    "validator",
				SubjectID:      strings.TrimSpace(item.VotePubkey),
				Source:         "solana-rpc:getVoteAccounts",
				ObservedAt:     observedAt,
				EvidenceStatus: "observed",
				StakeSharePct:  share,
			})
			if err != nil {
				return err
			}
			validators = append(validators, SolanaVoteAccountTelemetry{
				VotePubkey:     strings.TrimSpace(item.VotePubkey),
				NodePubkey:     strings.TrimSpace(item.NodePubkey),
				ActivatedStake: item.ActivatedStake,
				Commission:     item.Commission,
				LastVote:       item.LastVote,
				RootSlot:       item.RootSlot,
				Delinquent:     delinquent,
				Observation:    observation,
			})
		}
		return nil
	}
	if err := appendItems(result.Current, false); err != nil {
		return SolanaValidatorTelemetryReport{}, err
	}
	if err := appendItems(result.Delinquent, true); err != nil {
		return SolanaValidatorTelemetryReport{}, err
	}

	return SolanaValidatorTelemetryReport{
		SchemaVersion:       networktarget.NetworkTelemetrySchemaVersion,
		Network:             "solana-mainnet",
		CurrentCount:        len(result.Current),
		DelinquentCount:     len(result.Delinquent),
		TotalActivatedStake: total,
		Validators:          validators,
		Source:              "solana-rpc:getVoteAccounts",
		ObservedAt:          observedAt.UTC(),
		AnalysisPerformed:   true,
		LiveAvailability:    "checked",
	}, nil
}

func totalSolanaActivatedStake(groups ...[]solanaVoteAccountRPCItem) (uint64, error) {
	var total uint64
	for _, group := range groups {
		for _, item := range group {
			if ^uint64(0)-total < item.ActivatedStake {
				return 0, fmt.Errorf("solana_validator_telemetry_stake_overflow")
			}
			total += item.ActivatedStake
		}
	}
	return total, nil
}
