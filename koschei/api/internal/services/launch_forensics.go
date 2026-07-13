package services

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"
)

const ModuleLaunchForensics = "launch_forensics"

type LaunchForensicsAnalysis struct {
	Available              bool                  `json:"available"`
	Status                 string                `json:"status"`
	DataSource             string                `json:"data_source"`
	CapturedLive           bool                  `json:"captured_live"`
	LaunchSlot             int64                 `json:"launch_slot,omitempty"`
	LaunchTime             string                `json:"launch_time,omitempty"`
	LaunchTimeSource       string                `json:"launch_time_source,omitempty"`
	OwnersRequested        int                   `json:"owners_requested"`
	OwnersWithTradeHistory int                   `json:"owners_with_trade_history"`
	LedgerTradeCount       int                   `json:"ledger_trade_count"`
	ATASignatureLimit      int                   `json:"ata_signature_limit"`
	ATAMaxPages            int                   `json:"ata_max_pages"`
	RPCBudget              int                   `json:"rpc_budget"`
	RPCCallsUsed           int                   `json:"rpc_calls_used"`
	FundingRPCBudget       int                   `json:"funding_rpc_budget"`
	FundingRPCCallsUsed    int                   `json:"funding_rpc_calls_used"`
	FundingOwnersAttempted int                   `json:"funding_owners_attempted"`
	FundingOwnersResolved  int                   `json:"funding_owners_resolved"`
	SniperCount            int                   `json:"sniper_count"`
	RhythmBotCount         int                   `json:"rhythm_bot_count"`
	FlipperCount           int                   `json:"flipper_count"`
	AccumulatorCount       int                   `json:"accumulator_count"`
	CreatorLinkedCount     int                   `json:"creator_linked_count"`
	RiskContribution       int                   `json:"risk_contribution"`
	StructuralFloor        int                   `json:"structural_floor"`
	Profiles               []LaunchActorProfile  `json:"profiles"`
	Timeline               []LaunchTimelineEntry `json:"timeline"`
	Summary                string                `json:"summary"`
	Findings               []string              `json:"findings"`
	Limitations            []string              `json:"limitations"`
}

type launchOwnerCandidate struct {
	OwnerWallet   string
	TokenAccounts []string
	Balance       float64
	Rank          int
}

func AnalyzeLaunchForensics(ctx context.Context, db *sql.DB, rpcURL, mint, creator string, roles HolderRoleAnalysis, launchBlockTime, launchSlot int64) LaunchForensicsAnalysis {
	cfg := loadLaunchForensicsConfig()
	out := LaunchForensicsAnalysis{
		Status: "insufficient_evidence", DataSource: "none", LaunchSlot: launchSlot,
		ATASignatureLimit: cfg.ATASignatureLimit, ATAMaxPages: cfg.ATAMaxPages,
		RPCBudget: cfg.RPCBudget, FundingRPCBudget: cfg.FundingRPCBudget,
		Profiles: []LaunchActorProfile{}, Timeline: []LaunchTimelineEntry{}, Findings: []string{}, Limitations: []string{},
	}
	mint = strings.TrimSpace(mint)
	creator = strings.TrimSpace(creator)
	candidates := launchOwnerCandidates(roles.Accounts, 20)
	out.OwnersRequested = len(candidates)
	if mint == "" || len(candidates) == 0 {
		out.Limitations = append(out.Limitations, "Token mint and resolved risk-bearing holder owners are required for Launch Forensics.")
		return out
	}

	launchTime := time.Time{}
	if launchBlockTime > 0 {
		launchTime = time.Unix(launchBlockTime, 0).UTC()
		out.LaunchTimeSource = "mint_or_radar_history"
	}
	ledgerTrades, ledgerErr := loadTokenTradeEvents(ctx, db, mint, cfg.LedgerLimit)
	if ledgerErr != nil {
		out.Limitations = append(out.Limitations, "Live trade ledger could not be read; ATA history fallback was used.")
	}
	out.LedgerTradeCount = len(ledgerTrades)
	if len(ledgerTrades) > 0 {
		out.CapturedLive = true
		out.DataSource = "live_ledger"
		if launchSlot <= 0 || launchTime.IsZero() {
			launchSlot, launchTime = earliestLaunchObservation(ledgerTrades, launchSlot, launchTime)
			out.LaunchSlot = launchSlot
			out.LaunchTimeSource = "earliest_live_trade"
		}
	}

	allLedgerProfiles := classifyLaunchActors(ledgerTrades, launchSlot, launchTime, cfg.SniperSlotWindow)
	ledgerByWallet := map[string]LaunchActorProfile{}
	for _, profile := range allLedgerProfiles {
		ledgerByWallet[strings.TrimSpace(profile.OwnerWallet)] = profile
	}

	profileByWallet := map[string]LaunchActorProfile{}
	missing := make([]launchOwnerCandidate, 0, len(candidates))
	for _, candidate := range candidates {
		if profile, ok := ledgerByWallet[candidate.OwnerWallet]; ok && profile.TradeCount > 0 {
			profile.TokenAccounts = append([]string{}, candidate.TokenAccounts...)
			profile.Source = "live_ledger"
			profileByWallet[candidate.OwnerWallet] = profile
		} else {
			missing = append(missing, candidate)
		}
	}

	rpcBudget := newHolderScanRPCBudget(cfg.RPCBudget)
	if len(missing) > 0 && strings.TrimSpace(rpcURL) != "" && ctx.Err() == nil {
		historical := analyzeLaunchForensicsATA(ctx, rpcURL, mint, missing, launchTime, launchSlot, cfg, rpcBudget)
		for _, profile := range historical {
			if profile.TradeCount > 0 {
				profileByWallet[profile.OwnerWallet] = profile
			}
		}
		if len(historical) > 0 {
			if out.DataSource == "live_ledger" {
				out.DataSource = "hybrid_live_and_ata"
			} else {
				out.DataSource = "ata_history"
			}
		}
	}
	out.RPCCallsUsed = rpcBudget.Used()

	profiles := make([]LaunchActorProfile, 0, len(candidates))
	for _, candidate := range candidates {
		profile, ok := profileByWallet[candidate.OwnerWallet]
		if !ok {
			profile = LaunchActorProfile{
				OwnerWallet: candidate.OwnerWallet, TokenAccounts: append([]string{}, candidate.TokenAccounts...),
				Label: "HISTORY_NOT_CAPTURED", FundingStatus: "not_checked", Evidence: []string{"Bu owner için canlı ledger veya ATA işlem geçmişi üretilemedi; yokluk güvenlik sinyali sayılmadı."},
			}
		}
		profiles = append(profiles, profile)
	}

	if launchTime.IsZero() || launchSlot <= 0 {
		observedTrades := make([]LaunchTrade, 0)
		for _, profile := range profiles {
			if !profile.FirstBuyTime.IsZero() {
				observedTrades = append(observedTrades, LaunchTrade{Trader: profile.OwnerWallet, Side: "buy", Slot: profile.FirstBuySlot, BlockTime: profile.FirstBuyTime})
			}
		}
		launchSlot, launchTime = earliestLaunchObservation(observedTrades, launchSlot, launchTime)
		out.LaunchSlot = launchSlot
		if !launchTime.IsZero() && out.LaunchTimeSource == "" {
			out.LaunchTimeSource = "earliest_observed_top_holder_trade"
		}
		profiles = rerankLaunchProfiles(profiles, launchSlot, launchTime, cfg.SniperSlotWindow)
	}

	fundingBudget := newHolderScanRPCBudget(cfg.FundingRPCBudget)
	creatorFunder := ""
	if creator != "" && strings.TrimSpace(rpcURL) != "" {
		creatorFunder, _ = earliestIncomingSOLSource(ctx, rpcURL, creator, cfg, fundingBudget)
	}
	checks := cfg.FundingCheckCount
	if checks > len(profiles) {
		checks = len(profiles)
	}
	for i := 0; i < checks; i++ {
		out.FundingOwnersAttempted++
		traceLaunchFunding(ctx, rpcURL, creator, creatorFunder, &profiles[i], cfg, fundingBudget)
		if profiles[i].FundingStatus == "creator_linked" || profiles[i].FundingStatus == "funding_source_observed" {
			out.FundingOwnersResolved++
		}
	}
	for i := checks; i < len(profiles); i++ {
		profiles[i].FundingStatus = "not_traced_outside_top_limit"
	}
	out.FundingRPCCallsUsed = fundingBudget.Used()
	if len(allLedgerProfiles) > 0 {
		fundingByWallet := map[string]LaunchActorProfile{}
		for _, profile := range profiles {
			fundingByWallet[profile.OwnerWallet] = profile
		}
		for i := range allLedgerProfiles {
			if funded, ok := fundingByWallet[allLedgerProfiles[i].OwnerWallet]; ok {
				allLedgerProfiles[i].CreatorLinked = funded.CreatorLinked
				allLedgerProfiles[i].FundingStatus = funded.FundingStatus
				allLedgerProfiles[i].FundingHops = funded.FundingHops
				allLedgerProfiles[i].FundingPath = append([]string{}, funded.FundingPath...)
				if funded.CreatorLinked {
					allLedgerProfiles[i].Evidence = append(allLedgerProfiles[i].Evidence, funded.Evidence...)
				}
			}
		}
	}

	out.Profiles = profiles
	out.OwnersWithTradeHistory = countLaunchProfilesWithTrades(profiles)
	out.SniperCount, out.RhythmBotCount, out.FlipperCount, out.AccumulatorCount, out.CreatorLinkedCount = countLaunchLabels(profiles)
	out.RiskContribution, out.StructuralFloor = launchForensicsRisk(profiles)
	out.Timeline = launchTimeline(allLedgerProfiles, profiles, out.CapturedLive, 10)
	if !launchTime.IsZero() {
		out.LaunchTime = launchTime.UTC().Format(time.RFC3339)
	}
	if out.OwnersWithTradeHistory == 0 {
		out.Status = "launch_history_not_captured"
		out.Summary = "Launch window not captured — token predates live monitoring. Historical tiers apply."
		out.Limitations = append(out.Limitations, out.Summary)
		return out
	}
	out.Available = true
	out.Status = "verified_launch_forensics"
	out.Summary = launchForensicsSummary(out)
	out.Findings = launchForensicsFindings(out)
	if out.RPCCallsUsed >= out.RPCBudget {
		out.Limitations = append(out.Limitations, "ATA RPC budget was reached; partial owner histories were preserved and remaining gaps were not scored as safe.")
	}
	if out.FundingRPCCallsUsed >= out.FundingRPCBudget {
		out.Limitations = append(out.Limitations, "Funding trace budget was reached; remaining owners are marked funding not traced.")
	}
	return out
}

type launchForensicsConfig struct {
	ATASignatureLimit int
	ATAMaxPages       int
	RPCBudget         int
	FundingCheckCount int
	FundingRPCBudget  int
	SniperSlotWindow  int
	FundingSigLimit   int
	FundingTxParse    int
	LedgerLimit       int
}

func loadLaunchForensicsConfig() launchForensicsConfig {
	return launchForensicsConfig{
		ATASignatureLimit: holderScanEnvInt("ARVIS_ATA_SIG_LIMIT", 50, 10, 200),
		ATAMaxPages:       holderScanEnvInt("ARVIS_ATA_MAX_PAGES", 3, 1, 10),
		RPCBudget:         holderScanEnvInt("ARVIS_SCAN_RPC_BUDGET", 600, 25, 5000),
		FundingCheckCount: holderScanEnvInt("ARVIS_FUNDING_CHECK_COUNT", 10, 0, 20),
		FundingRPCBudget:  holderScanEnvInt("ARVIS_FUNDING_RPC_BUDGET", 100, 10, 1000),
		SniperSlotWindow:  holderScanEnvInt("ARVIS_SNIPER_SLOT_WINDOW", 3, 0, 100),
		FundingSigLimit:   holderScanEnvInt("ARVIS_FUNDING_SIG_LIMIT", 25, 5, 100),
		FundingTxParse:    holderScanEnvInt("ARVIS_FUNDING_TX_PARSE", 6, 1, 20),
		LedgerLimit:       launchEnvInt("ARVIS_TRADE_LEDGER_READ_LIMIT", 20000, 100, 100000),
	}
}

func launchEnvInt(name string, fallback, min, max int) int {
	raw := strings.TrimSpace(os.Getenv(name))
	if raw == "" {
		return fallback
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value < min || value > max {
		return fallback
	}
	return value
}

func launchOwnerCandidates(accounts []HolderRoleAccount, limit int) []launchOwnerCandidate {
	byOwner := map[string]*launchOwnerCandidate{}
	for _, account := range accounts {
		owner := strings.TrimSpace(account.OwnerWallet)
		if owner == "" || account.ExcludedFromHolderRisk || account.Role != "externally_owned_wallet" {
			continue
		}
		candidate := byOwner[owner]
		if candidate == nil {
			candidate = &launchOwnerCandidate{OwnerWallet: owner, Rank: account.Rank}
			byOwner[owner] = candidate
		}
		candidate.Balance += account.Balance
		if tokenAccount := strings.TrimSpace(account.TokenAccount); tokenAccount != "" && !containsLaunchString(candidate.TokenAccounts, tokenAccount) {
			candidate.TokenAccounts = append(candidate.TokenAccounts, tokenAccount)
		}
		if account.Rank > 0 && (candidate.Rank == 0 || account.Rank < candidate.Rank) {
			candidate.Rank = account.Rank
		}
	}
	out := make([]launchOwnerCandidate, 0, len(byOwner))
	for _, candidate := range byOwner {
		out = append(out, *candidate)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Balance > out[j].Balance })
	if len(out) > limit {
		out = out[:limit]
	}
	return out
}

func loadTokenTradeEvents(ctx context.Context, db *sql.DB, mint string, limit int) ([]LaunchTrade, error) {
	if db == nil {
		return nil, nil
	}
	rows, err := db.QueryContext(ctx, `
		SELECT mint,trader,side,sol_amount::float8,token_amount::float8,
		       COALESCE(slot,0),block_time,signature,source
		FROM token_trade_events
		WHERE mint=$1
		ORDER BY COALESCE(slot,0) ASC, COALESCE(block_time,created_at) ASC
		LIMIT $2`, mint, limit)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "token_trade_events") {
			return nil, nil
		}
		return nil, err
	}
	defer rows.Close()
	out := []LaunchTrade{}
	for rows.Next() {
		var trade LaunchTrade
		var blockTime sql.NullTime
		if err := rows.Scan(&trade.Mint, &trade.Trader, &trade.Side, &trade.SOLAmount, &trade.TokenAmount, &trade.Slot, &blockTime, &trade.Signature, &trade.Source); err != nil {
			return out, err
		}
		if blockTime.Valid {
			trade.BlockTime = blockTime.Time.UTC()
		}
		out = append(out, trade)
	}
	return out, rows.Err()
}

func earliestLaunchObservation(trades []LaunchTrade, slot int64, observed time.Time) (int64, time.Time) {
	for _, trade := range trades {
		if trade.Slot > 0 && (slot <= 0 || trade.Slot < slot) {
			slot = trade.Slot
		}
		if !trade.BlockTime.IsZero() && (observed.IsZero() || trade.BlockTime.Before(observed)) {
			observed = trade.BlockTime.UTC()
		}
	}
	return slot, observed
}

func rerankLaunchProfiles(profiles []LaunchActorProfile, launchSlot int64, launchTime time.Time, sniperWindow int) []LaunchActorProfile {
	trades := []LaunchTrade{}
	for _, profile := range profiles {
		if profile.TradeCount == 0 || profile.FirstBuyTime.IsZero() && profile.FirstBuySlot == 0 {
			continue
		}
		trades = append(trades, LaunchTrade{Trader: profile.OwnerWallet, Side: "buy", Slot: profile.FirstBuySlot, BlockTime: profile.FirstBuyTime, TokenAmount: profile.BoughtTokenAmount, Source: profile.Source})
	}
	ranks := classifyLaunchActors(trades, launchSlot, launchTime, sniperWindow)
	rankMap := map[string]LaunchActorProfile{}
	for _, rank := range ranks {
		rankMap[rank.OwnerWallet] = rank
	}
	for i := range profiles {
		if rank, ok := rankMap[profiles[i].OwnerWallet]; ok {
			profiles[i].EntryRank = rank.EntryRank
			profiles[i].SlotOffsetFromLaunch = rank.SlotOffsetFromLaunch
			profiles[i].MinutesAfterLaunch = rank.MinutesAfterLaunch
			profiles[i].LaunchSlotKnown = rank.LaunchSlotKnown
			profiles[i].LaunchTimeKnown = rank.LaunchTimeKnown
			if rank.Sniper && !profiles[i].Sniper {
				profiles[i].Sniper = true
				profiles[i].Evidence = append(profiles[i].Evidence, rank.Evidence...)
				if profiles[i].Label == LaunchLabelOrganic {
					profiles[i].Label = LaunchLabelSniperBot
				}
			}
		}
	}
	return profiles
}

func launchTimeline(allLedger, topProfiles []LaunchActorProfile, capturedLive bool, limit int) []LaunchTimelineEntry {
	source := topProfiles
	if capturedLive && len(allLedger) > 0 {
		source = allLedger
	}
	sorted := append([]LaunchActorProfile{}, source...)
	sort.SliceStable(sorted, func(i, j int) bool {
		if sorted[i].EntryRank == 0 {
			return false
		}
		if sorted[j].EntryRank == 0 {
			return true
		}
		return sorted[i].EntryRank < sorted[j].EntryRank
	})
	out := []LaunchTimelineEntry{}
	for _, profile := range sorted {
		if profile.EntryRank <= 0 || profile.BuyCount == 0 {
			continue
		}
		entry := LaunchTimelineEntry{
			EntryRank: profile.EntryRank, Wallet: profile.OwnerWallet, FirstBuySlot: profile.FirstBuySlot,
			SlotOffset: profile.SlotOffsetFromLaunch, MinutesAfterLaunch: profile.MinutesAfterLaunch,
			LaunchSlotKnown: profile.LaunchSlotKnown, LaunchTimeKnown: profile.LaunchTimeKnown,
			Label: profile.Label, CreatorLinked: profile.CreatorLinked, FundingHops: profile.FundingHops,
			Evidence: append([]string{}, profile.Evidence...),
		}
		if !profile.FirstBuyTime.IsZero() {
			entry.FirstBuyTime = profile.FirstBuyTime.UTC().Format(time.RFC3339)
		}
		out = append(out, entry)
		if len(out) >= limit {
			break
		}
	}
	return out
}

func countLaunchProfilesWithTrades(profiles []LaunchActorProfile) int {
	count := 0
	for _, profile := range profiles {
		if profile.TradeCount > 0 {
			count++
		}
	}
	return count
}

func countLaunchLabels(profiles []LaunchActorProfile) (sniper, rhythm, flipper, accumulator, linked int) {
	for _, profile := range profiles {
		if profile.Sniper {
			sniper++
		}
		if profile.RhythmBot {
			rhythm++
		}
		if profile.Flipper {
			flipper++
		}
		if profile.Accumulator {
			accumulator++
		}
		if profile.CreatorLinked {
			linked++
		}
	}
	return
}

func launchForensicsRisk(profiles []LaunchActorProfile) (int, int) {
	observed := countLaunchProfilesWithTrades(profiles)
	if observed == 0 {
		return 0, 0
	}
	sniper, rhythm, _, _, linked := countLaunchLabels(profiles)
	contribution := 0
	sniperPct := float64(sniper) / float64(observed) * 100
	switch {
	case sniperPct >= 50 && sniper >= 4:
		contribution += 32
	case sniperPct >= 30 && sniper >= 3:
		contribution += 22
	case sniper >= 2:
		contribution += 10
	}
	switch {
	case rhythm >= 4:
		contribution += 24
	case rhythm >= 2:
		contribution += 14
	case rhythm == 1:
		contribution += 5
	}
	switch {
	case linked >= 4:
		contribution += 38
	case linked >= 2:
		contribution += 24
	case linked == 1:
		contribution += 10
	}
	if sniper >= 3 && linked >= 2 {
		contribution += 12
	}
	if contribution > 90 {
		contribution = 90
	}
	floor := contribution
	if floor > 0 {
		floor += 5
	}
	if floor > 95 {
		floor = 95
	}
	return contribution, floor
}

func launchForensicsSummary(out LaunchForensicsAnalysis) string {
	return fmt.Sprintf("İlk %d risk taşıyan ownerın %d tanesinde hedef-token işlem geçmişi çözüldü. %d cüzdan lansman penceresinde sniper davranışı, %d cüzdan makine düzenine yakın işlem ritmi ve %d cüzdan creator fonlama zinciri bağlantısı üretti. Bu ilişkiler koordineli lansman riskini yükseltir; tek başına dolandırıcılık veya ortak gerçek kişi kanıtı değildir.", out.OwnersRequested, out.OwnersWithTradeHistory, out.SniperCount, out.RhythmBotCount, out.CreatorLinkedCount)
}

func launchForensicsFindings(out LaunchForensicsAnalysis) []string {
	findings := []string{out.Summary}
	if out.FlipperCount > 0 {
		findings = append(findings, fmt.Sprintf("%d holder alınan miktarın en az %%80'ini ilk 15 dakika içinde sattı.", out.FlipperCount))
	}
	if out.AccumulatorCount > 0 {
		findings = append(findings, fmt.Sprintf("%d holder en az 30 dakikaya yayılan, satışsız birikim davranışı gösterdi.", out.AccumulatorCount))
	}
	findings = append(findings, fmt.Sprintf("Lansman analizi RPC kullanımı: ATA %d/%d, fonlama %d/%d; fonlama izi çözülen owner %d/%d.", out.RPCCallsUsed, out.RPCBudget, out.FundingRPCCallsUsed, out.FundingRPCBudget, out.FundingOwnersResolved, out.FundingOwnersAttempted))
	return findings
}

func containsLaunchString(values []string, target string) bool {
	for _, value := range values {
		if strings.TrimSpace(value) == strings.TrimSpace(target) {
			return true
		}
	}
	return false
}
