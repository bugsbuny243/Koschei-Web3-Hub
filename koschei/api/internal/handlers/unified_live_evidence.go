package handlers

import (
	"context"
	"math"
	"sort"
	"strings"
	"time"

	"koschei/api/internal/services"
)

const (
	unifiedLiveSignatureLimit   = 30
	unifiedLiveTransactionLimit = 12
	unifiedLaunchSignatureLimit = 100
	unifiedLaunchParseLimit     = 16
)

type unifiedLiveWalletTarget struct {
	Wallet string `json:"wallet"`
	Role   string `json:"role"`
}

type unifiedLiveWalletCoverage struct {
	Wallet               string   `json:"wallet"`
	Role                 string   `json:"role"`
	Status               string   `json:"status"`
	SignaturesSeen       int      `json:"signatures_seen"`
	TransactionsParsed   int      `json:"transactions_parsed"`
	RelevantTransactions int      `json:"relevant_transactions"`
	RPCFailures          int      `json:"rpc_failures"`
	Limitations          []string `json:"limitations"`
}

type unifiedLaunchSignerObservation struct {
	Available      bool     `json:"available"`
	Status         string   `json:"status"`
	Wallet         string   `json:"wallet,omitempty"`
	Signature      string   `json:"signature,omitempty"`
	Slot           int64    `json:"slot,omitempty"`
	ObservedAt     string   `json:"observed_at,omitempty"`
	InstructionTypes []string `json:"instruction_types"`
	EvidenceKey    string   `json:"evidence_key,omitempty"`
	Limitations    []string `json:"limitations"`
}

type unifiedLiveInvestigationReport struct {
	Status             string                          `json:"status"`
	Mint               string                          `json:"mint"`
	RPCConfigured      bool                            `json:"rpc_configured"`
	WalletsRequested   int                             `json:"wallets_requested"`
	WalletsCompleted   int                             `json:"wallets_completed"`
	SignaturesSeen     int                             `json:"signatures_seen"`
	TransactionsParsed int                             `json:"transactions_parsed"`
	RelevantTransactions int                           `json:"relevant_transactions"`
	RPCFailures        int                             `json:"rpc_failures"`
	LaunchSigner       unifiedLaunchSignerObservation  `json:"launch_signer"`
	WalletCoverage     []unifiedLiveWalletCoverage     `json:"wallet_coverage"`
	Transactions       []unifiedTransactionEvidence    `json:"transactions"`
	GeneratedAt        time.Time                       `json:"generated_at"`
	Limitations        []string                        `json:"limitations"`
}

func unifiedLiveEvidenceAllowed(mode string) bool {
	value := strings.ToLower(strings.TrimSpace(mode))
	if value == "" {
		return false
	}
	return !strings.Contains(value, "preflight") && !strings.Contains(value, "safe_check") && !strings.Contains(value, "safe-check") && !strings.Contains(value, "stored_only")
}

func (h *Handler) collectUnifiedTokenLiveEvidence(ctx context.Context, core holderIntelligenceCoreResult) unifiedLiveInvestigationReport {
	now := time.Now().UTC()
	out := unifiedLiveInvestigationReport{
		Status: "source_unavailable", Mint: strings.TrimSpace(core.Request.Target),
		WalletCoverage: []unifiedLiveWalletCoverage{}, Transactions: []unifiedTransactionEvidence{},
		GeneratedAt: now, Limitations: []string{},
		LaunchSigner: unifiedLaunchSignerObservation{Status: "not_checked", InstructionTypes: []string{}, Limitations: []string{}},
	}
	rpcURL := strings.TrimSpace(creatorIntelRPCURL())
	if rpcURL == "" {
		out.Limitations = append(out.Limitations, "Solana RPC is unavailable; no live transaction rows were collected.")
		return out
	}
	out.RPCConfigured = true

	creator := strings.TrimSpace(creatorIntelCleanString(core.SourceContext["creator_wallet"]))
	launchSigner := unifiedLaunchSignerObservation{Status: "not_required", InstructionTypes: []string{}, Limitations: []string{}}
	if creator == "" {
		launchCtx, cancel := context.WithTimeout(ctx, 24*time.Second)
		launchSigner = discoverUnifiedLaunchSigner(launchCtx, rpcURL, out.Mint)
		cancel()
	}
	out.LaunchSigner = launchSigner

	targets := unifiedLiveWalletTargets(core.Intelligence, creator, launchSigner)
	out.WalletsRequested = len(targets)
	if len(targets) == 0 {
		out.Status = "no_resolved_wallet_targets"
		out.Limitations = append(out.Limitations, "No creator, launch signer or owner-resolved risk-bearing wallet was available for live transaction inspection.")
		return out
	}

	for _, target := range targets {
		if ctx.Err() != nil {
			out.Status = "partial_timeout"
			out.Limitations = append(out.Limitations, "The bounded live transaction window ended before every wallet target completed.")
			break
		}
		walletCtx, cancel := context.WithTimeout(ctx, 28*time.Second)
		coverage, rows := collectUnifiedWalletTransactions(walletCtx, rpcURL, out.Mint, target)
		cancel()
		out.WalletCoverage = append(out.WalletCoverage, coverage)
		out.SignaturesSeen += coverage.SignaturesSeen
		out.TransactionsParsed += coverage.TransactionsParsed
		out.RelevantTransactions += coverage.RelevantTransactions
		out.RPCFailures += coverage.RPCFailures
		if coverage.Status == "complete" || coverage.Status == "complete_no_relevant_token_delta" {
			out.WalletsCompleted++
		}
		out.Transactions = append(out.Transactions, rows...)
	}
	out.Transactions = normalizeUnifiedTransactionEvidence(out.Transactions)
	out.RelevantTransactions = len(out.Transactions)
	if out.WalletsCompleted == out.WalletsRequested {
		out.Status = "complete"
	} else if out.WalletsCompleted > 0 || len(out.Transactions) > 0 {
		out.Status = "partial"
	} else if out.Status == "source_unavailable" {
		out.Status = "collection_failed"
	}
	out.Limitations = append(out.Limitations,
		"Live transaction inspection is bounded to recent wallet signatures and successful JSON-parsed transactions.",
		"A missing row means no relevant mint delta was observed in the bounded window; it is not proof that no older activity exists.",
	)
	return out
}

func unifiedLiveWalletTargets(holder services.HolderIntelligence, creator string, launch unifiedLaunchSignerObservation) []unifiedLiveWalletTarget {
	out := []unifiedLiveWalletTarget{}
	seen := map[string]bool{}
	add := func(wallet, role string) {
		wallet = strings.TrimSpace(wallet)
		if wallet == "" || seen[wallet] {
			return
		}
		seen[wallet] = true
		out = append(out, unifiedLiveWalletTarget{Wallet: wallet, Role: role})
	}
	add(creator, "creator_source_observed")
	if creator == "" && launch.Available {
		add(launch.Wallet, "launch_signer_observed")
	}
	ownerCount := 0
	for _, row := range holder.Rows {
		if ownerCount >= 3 {
			break
		}
		if !row.OwnerResolved || !row.RiskBearing || row.ExcludedFromHolderRisk {
			continue
		}
		wallet := strings.TrimSpace(row.OwnerWallet)
		if wallet == "" {
			continue
		}
		before := len(out)
		add(wallet, "risk_bearing_holder")
		if len(out) > before {
			ownerCount++
		}
	}
	return out
}

func collectUnifiedWalletTransactions(ctx context.Context, rpcURL, mint string, target unifiedLiveWalletTarget) (unifiedLiveWalletCoverage, []unifiedTransactionEvidence) {
	coverage := unifiedLiveWalletCoverage{
		Wallet: target.Wallet, Role: target.Role, Status: "rpc_failed", Limitations: []string{},
	}
	rows := []unifiedTransactionEvidence{}
	signatures, err := services.SolanaGetSignaturesForAddress(ctx, rpcURL, target.Wallet, unifiedLiveSignatureLimit)
	if err != nil {
		coverage.RPCFailures++
		coverage.Limitations = append(coverage.Limitations, "Wallet signatures could not be read from the configured Solana RPC provider.")
		return coverage, rows
	}
	coverage.SignaturesSeen = len(signatures)
	keys := []string{}
	infoBySignature := map[string]services.SolanaSignatureInfo{}
	for _, item := range signatures {
		if item.Err != nil || strings.TrimSpace(item.Signature) == "" {
			continue
		}
		keys = append(keys, item.Signature)
		infoBySignature[item.Signature] = item
		if len(keys) >= unifiedLiveTransactionLimit {
			break
		}
	}
	if len(keys) == 0 {
		coverage.Status = "complete_no_successful_signatures"
		return coverage, rows
	}
	transactions, batchErr := fetchUnifiedTransactions(ctx, rpcURL, keys)
	if batchErr != nil {
		coverage.RPCFailures++
		coverage.Limitations = append(coverage.Limitations, "Some recent transactions could not be parsed: "+creatorIntelCompactError(batchErr))
	}
	for _, signature := range keys {
		tx, ok := transactions[signature]
		if !ok {
			continue
		}
		coverage.TransactionsParsed++
		if row, relevant := parseUnifiedLiveTransaction(mint, target, infoBySignature[signature], tx); relevant {
			rows = append(rows, row)
		}
	}
	coverage.RelevantTransactions = len(rows)
	coverage.Status = "complete"
	if len(rows) == 0 {
		coverage.Status = "complete_no_relevant_token_delta"
	}
	return coverage, rows
}

func fetchUnifiedTransactions(ctx context.Context, rpcURL string, signatures []string) (map[string]services.SolanaTransactionResult, error) {
	out, err := services.SolanaGetTransactionsJSONParsedBatch(ctx, rpcURL, signatures)
	if err == nil || len(out) > 0 {
		return out, err
	}
	out = map[string]services.SolanaTransactionResult{}
	var lastErr error
	for _, signature := range signatures {
		if ctx.Err() != nil {
			break
		}
		tx, singleErr := services.SolanaGetTransactionJSONParsed(ctx, rpcURL, signature)
		if singleErr != nil {
			lastErr = singleErr
			continue
		}
		if tx != nil {
			out[signature] = tx
		}
	}
	return out, lastErr
}

func parseUnifiedLiveTransaction(mint string, target unifiedLiveWalletTarget, signature services.SolanaSignatureInfo, tx services.SolanaTransactionResult) (unifiedTransactionEvidence, bool) {
	txMap := map[string]any(tx)
	meta := creatorIntelMap(txMap["meta"])
	if meta["err"] != nil {
		return unifiedTransactionEvidence{}, false
	}
	message := creatorIntelMap(creatorIntelMap(txMap["transaction"])["message"])
	instructionTypes, instructionMints := creatorIntelInstructions(message, meta)
	logs := strings.ToLower(strings.Join(creatorIntelStringSlice(meta["logMessages"]), "\n"))
	deltas := creatorIntelOwnerTokenDeltas(meta, mint)
	delta := deltas[target.Wallet]
	if math.Abs(delta) < 0.000000001 {
		return unifiedTransactionEvidence{}, false
	}
	swapRelated := creatorIntelSwapRelated(logs, instructionTypes)
	direction := "transfer_in"
	if delta < 0 {
		direction = "transfer_out"
		if swapRelated {
			direction = "sell"
		}
	} else if swapRelated {
		direction = "buy"
	}
	counterparties := []string{}
	for wallet, otherDelta := range deltas {
		wallet = strings.TrimSpace(wallet)
		if wallet == "" || wallet == target.Wallet || math.Abs(otherDelta) < 0.000000001 {
			continue
		}
		if (delta < 0 && otherDelta > 0) || (delta > 0 && otherDelta < 0) {
			counterparties = append(counterparties, wallet)
		}
	}
	sort.Strings(counterparties)
	blockTime := time.Time{}
	if raw := creatorIntelInt64(txMap["blockTime"]); raw > 0 {
		blockTime = time.Unix(raw, 0).UTC()
	} else if signature.BlockTime != nil && *signature.BlockTime > 0 {
		blockTime = time.Unix(*signature.BlockTime, 0).UTC()
	}
	row := unifiedTransactionEvidence{
		Signature: strings.TrimSpace(signature.Signature), Slot: signature.Slot,
		Trader: strings.TrimSpace(target.Wallet), Direction: direction, Role: target.Role,
		TokenDelta: creatorIntelRound(delta, 8), SwapRelated: swapRelated,
		Counterparties: counterparties, InstructionTypes: instructionTypes,
		Source: "solana_jsonparsed_manual_full_scan",
	}
	if !blockTime.IsZero() {
		row.BlockTime = &blockTime
	}
	row.EvidenceKey = row.Signature + ":" + row.Trader + ":" + row.Direction
	if len(instructionMints) > 0 {
		row.TokenMints = instructionMints
	}
	return row, row.Signature != ""
}

func discoverUnifiedLaunchSigner(ctx context.Context, rpcURL, mint string) unifiedLaunchSignerObservation {
	out := unifiedLaunchSignerObservation{Status: "not_observed", InstructionTypes: []string{}, Limitations: []string{}}
	signatures, err := services.SolanaGetSignaturesForAddress(ctx, rpcURL, mint, unifiedLaunchSignatureLimit)
	if err != nil {
		out.Status = "rpc_failed"
		out.Limitations = append(out.Limitations, "Mint signature history could not be read for launch-signer discovery.")
		return out
	}
	keys := []string{}
	info := map[string]services.SolanaSignatureInfo{}
	for index := len(signatures) - 1; index >= 0 && len(keys) < unifiedLaunchParseLimit; index-- {
		item := signatures[index]
		if item.Err != nil || strings.TrimSpace(item.Signature) == "" {
			continue
		}
		keys = append(keys, item.Signature)
		info[item.Signature] = item
	}
	transactions, fetchErr := fetchUnifiedTransactions(ctx, rpcURL, keys)
	if fetchErr != nil && len(transactions) == 0 {
		out.Status = "rpc_failed"
		out.Limitations = append(out.Limitations, "Launch-window transactions could not be parsed.")
		return out
	}
	for _, signature := range keys {
		tx, ok := transactions[signature]
		if !ok {
			continue
		}
		txMap := map[string]any(tx)
		meta := creatorIntelMap(txMap["meta"])
		if meta["err"] != nil {
			continue
		}
		message := creatorIntelMap(creatorIntelMap(txMap["transaction"])["message"])
		types, mints := creatorIntelInstructions(message, meta)
		logs := strings.ToLower(strings.Join(creatorIntelStringSlice(meta["logMessages"]), "\n"))
		if !creatorIntelLaunchRelated(logs, types) || !unifiedTransactionReferencesMint(meta, mints, mint) {
			continue
		}
		_, signers := creatorIntelAccountKeys(message)
		wallet := ""
		for _, signer := range signers {
			if strings.TrimSpace(signer) != "" && strings.TrimSpace(signer) != mint {
				wallet = strings.TrimSpace(signer)
				break
			}
		}
		if wallet == "" {
			continue
		}
		item := info[signature]
		observedAt := ""
		if raw := creatorIntelInt64(txMap["blockTime"]); raw > 0 {
			observedAt = time.Unix(raw, 0).UTC().Format(time.RFC3339)
		} else if item.BlockTime != nil && *item.BlockTime > 0 {
			observedAt = time.Unix(*item.BlockTime, 0).UTC().Format(time.RFC3339)
		}
		out.Available = true
		out.Status = "observed_launch_transaction_signer"
		out.Wallet = wallet
		out.Signature = signature
		out.Slot = item.Slot
		out.ObservedAt = observedAt
		out.InstructionTypes = types
		out.EvidenceKey = signature + ":launch_signer:" + wallet
		out.Limitations = append(out.Limitations, "The launch transaction signer is an on-chain relation, not a real-world identity or guaranteed creator attribution.")
		return out
	}
	out.Limitations = append(out.Limitations, "No launch-marked transaction signer was resolved in the bounded mint signature window.")
	return out
}

func unifiedTransactionReferencesMint(meta map[string]any, instructionMints []string, mint string) bool {
	for _, value := range instructionMints {
		if strings.TrimSpace(value) == mint {
			return true
		}
	}
	for _, key := range []string{"preTokenBalances", "postTokenBalances"} {
		items, _ := meta[key].([]any)
		for _, raw := range items {
			if creatorIntelCleanString(creatorIntelMap(raw)["mint"]) == mint {
				return true
			}
		}
	}
	return false
}

func normalizeUnifiedTransactionEvidence(values []unifiedTransactionEvidence) []unifiedTransactionEvidence {
	seen := map[string]bool{}
	out := []unifiedTransactionEvidence{}
	for _, value := range values {
		key := strings.TrimSpace(value.Signature) + "|" + strings.TrimSpace(value.Trader) + "|" + strings.TrimSpace(value.Direction)
		if key == "||" || seen[key] {
			continue
		}
		seen[key] = true
		value.Counterparties = uniqueStringsSorted(value.Counterparties)
		value.InstructionTypes = uniqueStringsSorted(value.InstructionTypes)
		value.TokenMints = uniqueStringsSorted(value.TokenMints)
		out = append(out, value)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Slot == out[j].Slot {
			return out[i].Signature < out[j].Signature
		}
		return out[i].Slot > out[j].Slot
	})
	return out
}

func mergeUnifiedTransactionEvidence(stored, live []unifiedTransactionEvidence) []unifiedTransactionEvidence {
	all := append([]unifiedTransactionEvidence{}, stored...)
	all = append(all, live...)
	return normalizeUnifiedTransactionEvidence(all)
}

func summarizeUnifiedTransactionEvidence(values []unifiedTransactionEvidence) map[string]any {
	out := map[string]any{
		"available": len(values) > 0, "status": "bounded_live_transaction_window",
		"trade_count": int64(len(values)), "buy_count": int64(0), "sell_count": int64(0),
		"transfer_in_count": int64(0), "transfer_out_count": int64(0),
		"unique_trader_count": int64(0), "round_trip_wallet_count": int64(0),
		"wash_classification": "not_proven",
		"interpretation": "Counts are bounded live wallet observations; they are not complete market-wide trade history.",
	}
	walletSides := map[string]map[string]bool{}
	for _, row := range values {
		wallet := strings.TrimSpace(row.Trader)
		if wallet != "" {
			if walletSides[wallet] == nil { walletSides[wallet] = map[string]bool{} }
			walletSides[wallet][row.Direction] = true
		}
		switch row.Direction {
		case "buy": out["buy_count"] = out["buy_count"].(int64)+1
		case "sell": out["sell_count"] = out["sell_count"].(int64)+1
		case "transfer_in": out["transfer_in_count"] = out["transfer_in_count"].(int64)+1
		case "transfer_out": out["transfer_out_count"] = out["transfer_out_count"].(int64)+1
		}
	}
	roundTrip := int64(0)
	for _, sides := range walletSides {
		if (sides["buy"] || sides["transfer_in"]) && (sides["sell"] || sides["transfer_out"]) { roundTrip++ }
	}
	out["unique_trader_count"] = int64(len(walletSides))
	out["round_trip_wallet_count"] = roundTrip
	return out
}
