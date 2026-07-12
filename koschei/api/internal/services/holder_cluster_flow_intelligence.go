package services

import (
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
)

const holderClusterFlowEpsilon = 0.000000001

// HolderClusterFlowObservation records one evidence-scoped target-token outflow.
// It never claims real-world identity, ownership or wash trading on its own.
type HolderClusterFlowObservation struct {
	SourceWallet string   `json:"source_wallet"`
	Destination  string   `json:"destination"`
	Kind         string   `json:"kind"`
	Amount       float64  `json:"amount"`
	Slot         int64    `json:"slot,omitempty"`
	Signature    string   `json:"signature,omitempty"`
	ProgramIDs   []string `json:"program_ids"`
	Evidence     []string `json:"evidence"`
}

// HolderClusterFlowAnalysis summarizes direct holder transfers and repeated
// token exit destinations observed inside the bounded transaction window.
type HolderClusterFlowAnalysis struct {
	Available               bool                           `json:"available"`
	Status                  string                         `json:"status"`
	Confidence              string                         `json:"confidence"`
	TransactionsWithOutflow int                            `json:"transactions_with_outflow"`
	WalletsWithOutflow      int                            `json:"wallets_with_outflow"`
	DEXExitObservationCount int                            `json:"dex_exit_observation_count"`
	CommonExitGroupCount    int                            `json:"common_exit_group_count"`
	LargestCommonExitGroup  int                            `json:"largest_common_exit_group"`
	InternalTransferCount   int                            `json:"internal_transfer_count"`
	CircularWalletCount     int                            `json:"circular_wallet_count"`
	LinkedHolderPercentage  float64                        `json:"linked_holder_percentage"`
	RiskContribution        int                            `json:"risk_contribution"`
	LinkedWallets           []string                       `json:"linked_wallets"`
	CircularWallets         []string                       `json:"circular_wallets"`
	CommonExitGroups        []HolderClusterGroup           `json:"common_exit_groups"`
	InternalTransfers       []HolderClusterFlowObservation `json:"internal_transfers"`
	Observations            []HolderClusterFlowObservation `json:"observations"`
	Findings                []string                       `json:"findings"`
	Limitations             []string                       `json:"limitations"`
}

func observeHolderClusterWalletFlow(tx map[string]any, signature, mint, sourceWallet string, holderWallets map[string]bool) []HolderClusterFlowObservation {
	deltas := holderClusterTokenOwnerDeltas(tx, mint)
	sourceDelta := deltas[sourceWallet]
	if sourceDelta >= -holderClusterFlowEpsilon {
		return []HolderClusterFlowObservation{}
	}

	programIDs := holderClusterKnownExitPrograms(tx)
	slot := holderClusterInt64(tx["slot"])
	out := []HolderClusterFlowObservation{}
	seen := map[string]bool{}
	for destination, delta := range deltas {
		if strings.EqualFold(destination, sourceWallet) || delta <= holderClusterFlowEpsilon {
			continue
		}
		kind := "external_token_recipient"
		if holderWallets[destination] {
			kind = "holder_to_holder"
		}
		amount := math.Min(-sourceDelta, delta)
		key := kind + "|" + destination + "|" + signature
		if seen[key] {
			continue
		}
		seen[key] = true
		observation := HolderClusterFlowObservation{
			SourceWallet: sourceWallet,
			Destination:  destination,
			Kind:         kind,
			Amount:       holderClusterRound(amount, 9),
			Slot:         slot,
			Signature:    signature,
			ProgramIDs:   append([]string{}, programIDs...),
			Evidence: []string{
				fmt.Sprintf("Target-token balance decreased for %s while %s increased in the same parsed transaction.", sourceWallet, destination),
			},
		}
		if len(programIDs) > 0 {
			observation.Evidence = append(observation.Evidence, "Known pool/DEX program context was present; this is route context, not proof of a sale or common ownership.")
		}
		out = append(out, observation)
	}

	// A known pool program with a negative owner delta is useful route context,
	// even when RPC token-balance rows do not expose a recipient owner. Program
	// identity alone is deliberately excluded from common-exit scoring.
	if len(out) == 0 && len(programIDs) > 0 {
		out = append(out, HolderClusterFlowObservation{
			SourceWallet: sourceWallet,
			Destination:  programIDs[0],
			Kind:         "dex_program_exit_context",
			Amount:       holderClusterRound(-sourceDelta, 9),
			Slot:         slot,
			Signature:    signature,
			ProgramIDs:   append([]string{}, programIDs...),
			Evidence: []string{
				"A negative target-token balance delta was observed with known pool/DEX program context, but no recipient owner was exposed by the bounded RPC response.",
			},
		})
	}
	return out
}

func summarizeHolderClusterFlow(wallets []HolderClusterWallet) HolderClusterFlowAnalysis {
	out := HolderClusterFlowAnalysis{
		Status:            "insufficient_evidence",
		Confidence:        "none",
		LinkedWallets:     []string{},
		CircularWallets:   []string{},
		CommonExitGroups:  []HolderClusterGroup{},
		InternalTransfers: []HolderClusterFlowObservation{},
		Observations:      []HolderClusterFlowObservation{},
		Findings:          []string{},
		Limitations:       []string{},
	}
	walletPct := map[string]float64{}
	holderSet := map[string]bool{}
	for _, wallet := range wallets {
		walletPct[wallet.Wallet] = wallet.HolderPercentage
		holderSet[wallet.Wallet] = true
		out.Observations = append(out.Observations, wallet.FlowObservations...)
	}
	if len(out.Observations) == 0 {
		out.Limitations = append(out.Limitations, "No parsed target-token outflow relation was observed in the bounded holder transaction window.")
		return out
	}

	out.Available = true
	out.Status = "verified_bounded_holder_flow_observation"
	outgoingWallets := map[string]bool{}
	outflowTransactions := map[string]bool{}
	commonExitSources := map[string]map[string]bool{}
	internalSeen := map[string]bool{}
	for _, observation := range out.Observations {
		outgoingWallets[observation.SourceWallet] = true
		if observation.Signature != "" {
			outflowTransactions[observation.Signature] = true
		}
		if len(observation.ProgramIDs) > 0 {
			out.DEXExitObservationCount++
		}
		switch observation.Kind {
		case "holder_to_holder":
			key := observation.SourceWallet + "|" + observation.Destination + "|" + observation.Signature
			if !internalSeen[key] {
				internalSeen[key] = true
				out.InternalTransfers = append(out.InternalTransfers, observation)
			}
		case "external_token_recipient":
			if commonExitSources[observation.Destination] == nil {
				commonExitSources[observation.Destination] = map[string]bool{}
			}
			commonExitSources[observation.Destination][observation.SourceWallet] = true
		}
	}
	out.WalletsWithOutflow = len(outgoingWallets)
	out.TransactionsWithOutflow = len(outflowTransactions)
	out.InternalTransferCount = len(out.InternalTransfers)

	for destination, sourceSet := range commonExitSources {
		if len(sourceSet) < 2 {
			continue
		}
		wallets := make([]string, 0, len(sourceSet))
		groupPct := 0.0
		for wallet := range sourceSet {
			wallets = append(wallets, wallet)
			groupPct += walletPct[wallet]
		}
		sort.Strings(wallets)
		group := HolderClusterGroup{
			Key:              destination,
			Wallets:          wallets,
			MemberCount:      len(wallets),
			HolderPercentage: holderClusterRound(groupPct, 4),
			Evidence: []string{
				fmt.Sprintf("%d holder wallets sent the target token to the same observed recipient owner in bounded parsed transactions.", len(wallets)),
			},
		}
		out.CommonExitGroups = append(out.CommonExitGroups, group)
		if group.MemberCount > out.LargestCommonExitGroup {
			out.LargestCommonExitGroup = group.MemberCount
		}
	}
	sort.SliceStable(out.CommonExitGroups, func(i, j int) bool {
		if out.CommonExitGroups[i].MemberCount == out.CommonExitGroups[j].MemberCount {
			return out.CommonExitGroups[i].HolderPercentage > out.CommonExitGroups[j].HolderPercentage
		}
		return out.CommonExitGroups[i].MemberCount > out.CommonExitGroups[j].MemberCount
	})
	out.CommonExitGroupCount = len(out.CommonExitGroups)
	out.CircularWallets = holderClusterCircularWallets(out.InternalTransfers)
	out.CircularWalletCount = len(out.CircularWallets)

	linked := map[string]bool{}
	for _, group := range out.CommonExitGroups {
		for _, wallet := range group.Wallets {
			linked[wallet] = true
		}
	}
	for _, observation := range out.InternalTransfers {
		linked[observation.SourceWallet] = true
		if holderSet[observation.Destination] {
			linked[observation.Destination] = true
		}
	}
	for _, wallet := range out.CircularWallets {
		linked[wallet] = true
	}
	for wallet := range linked {
		out.LinkedWallets = append(out.LinkedWallets, wallet)
		out.LinkedHolderPercentage += walletPct[wallet]
	}
	sort.Strings(out.LinkedWallets)
	out.LinkedHolderPercentage = holderClusterRound(out.LinkedHolderPercentage, 4)

	score := 0
	switch {
	case out.LargestCommonExitGroup >= 4:
		score += 22
	case out.LargestCommonExitGroup >= 3:
		score += 15
	case out.LargestCommonExitGroup >= 2:
		score += 7
	}
	switch {
	case out.InternalTransferCount >= 4:
		score += 16
	case out.InternalTransferCount >= 2:
		score += 10
	case out.InternalTransferCount >= 1:
		score += 4
	}
	if out.CircularWalletCount >= 2 {
		score += 18
	}
	if out.LargestCommonExitGroup >= 3 && out.InternalTransferCount >= 2 {
		score += 10
	}
	if score > 45 {
		score = 45
	}
	out.RiskContribution = score
	switch {
	case out.CircularWalletCount >= 2 && out.LargestCommonExitGroup >= 2:
		out.Confidence = "high"
	case out.LargestCommonExitGroup >= 2 || out.InternalTransferCount >= 2 || out.CircularWalletCount >= 2:
		out.Confidence = "medium"
	default:
		out.Confidence = "low"
	}
	out.Findings = holderClusterFlowFindings(out)
	out.Limitations = append(out.Limitations,
		"Common exit means the same parsed target-token recipient owner was observed; an exchange, pool authority or service wallet can produce the same pattern.",
		"Known DEX/pool program presence is retained as route context but is not scored as common control by itself.",
		"Circular holder-to-holder transfer relations are not labelled wash trading without repeated swap, consideration and return-flow evidence.",
	)
	return out
}

func holderClusterFlowFindings(out HolderClusterFlowAnalysis) []string {
	findings := []string{fmt.Sprintf("Observed %d target-token outflow transactions across %d holder wallets in the bounded window.", out.TransactionsWithOutflow, out.WalletsWithOutflow)}
	if out.LargestCommonExitGroup >= 2 {
		findings = append(findings, fmt.Sprintf("The largest verified common-exit recipient group contains %d holder wallets.", out.LargestCommonExitGroup))
	}
	if out.InternalTransferCount > 0 {
		findings = append(findings, fmt.Sprintf("%d direct target-token transfer relations were observed between resolved holder wallets.", out.InternalTransferCount))
	}
	if out.CircularWalletCount >= 2 {
		findings = append(findings, fmt.Sprintf("%d holder wallets participate in a bounded circular transfer relation; this is not, by itself, a wash-trading verdict.", out.CircularWalletCount))
	}
	if out.LinkedHolderPercentage > 0 {
		findings = append(findings, fmt.Sprintf("Flow-linked holder wallets represent approximately %.4f%% of role-adjusted holder supply.", out.LinkedHolderPercentage))
	}
	if len(findings) == 1 {
		findings = append(findings, "No repeated common-exit, holder-to-holder or circular transfer pattern was verified in the bounded window.")
	}
	return findings
}

func holderClusterCircularWallets(observations []HolderClusterFlowObservation) []string {
	adjacency := map[string]map[string]bool{}
	for _, observation := range observations {
		if observation.Kind != "holder_to_holder" || observation.SourceWallet == "" || observation.Destination == "" {
			continue
		}
		if adjacency[observation.SourceWallet] == nil {
			adjacency[observation.SourceWallet] = map[string]bool{}
		}
		adjacency[observation.SourceWallet][observation.Destination] = true
	}
	circular := map[string]bool{}
	for start, nextSet := range adjacency {
		for next := range nextSet {
			if holderClusterFlowPathExists(adjacency, next, start, map[string]bool{start: true}) {
				circular[start] = true
				circular[next] = true
			}
		}
	}
	out := make([]string, 0, len(circular))
	for wallet := range circular {
		out = append(out, wallet)
	}
	sort.Strings(out)
	return out
}

func holderClusterFlowPathExists(adjacency map[string]map[string]bool, current, target string, visited map[string]bool) bool {
	if current == target {
		return true
	}
	if visited[current] {
		return false
	}
	visited[current] = true
	for next := range adjacency[current] {
		if holderClusterFlowPathExists(adjacency, next, target, visited) {
			return true
		}
	}
	return false
}

func holderClusterTokenOwnerDeltas(tx map[string]any, mint string) map[string]float64 {
	meta := holderClusterMap(tx["meta"])
	pre := holderClusterTokenOwnerTotals(meta["preTokenBalances"], mint)
	post := holderClusterTokenOwnerTotals(meta["postTokenBalances"], mint)
	out := map[string]float64{}
	for owner, amount := range pre {
		out[owner] -= amount
	}
	for owner, amount := range post {
		out[owner] += amount
	}
	return out
}

func holderClusterTokenOwnerTotals(raw any, mint string) map[string]float64 {
	out := map[string]float64{}
	for _, item := range holderClusterSlice(raw) {
		row := holderClusterMap(item)
		if !strings.EqualFold(holderClusterString(row["mint"]), mint) {
			continue
		}
		owner := strings.TrimSpace(holderClusterString(row["owner"]))
		if owner == "" {
			continue
		}
		amount := holderClusterMap(row["uiTokenAmount"])
		value := holderClusterFloat(amount["uiAmount"])
		if text := holderClusterString(amount["uiAmountString"]); text != "" {
			if parsed, err := strconvParseFloat(text); err == nil {
				value = parsed
			}
		}
		out[owner] += value
	}
	return out
}

func holderClusterKnownExitPrograms(tx map[string]any) []string {
	evidence := arvisTransactionEvidence{TokenBalanceChanges: map[string]float64{}, LamportDeltas: map[string]int64{}}
	parseArvisTransactionMap(tx, &evidence)
	seen := map[string]bool{}
	out := []string{}
	for _, programID := range evidence.ProgramIDs {
		if !isKnownRaydiumProgram(programID) && programID != pumpBondingCurveProgramID && programID != pumpLiquidityProgramID {
			continue
		}
		if !seen[programID] {
			seen[programID] = true
			out = append(out, programID)
		}
	}
	sort.Strings(out)
	return out
}

// Small wrapper keeps parsing behavior aligned with the rest of the service
// without exposing strconv details throughout the flow engine.
func strconvParseFloat(value string) (float64, error) {
	return strconv.ParseFloat(strings.TrimSpace(value), 64)
}
