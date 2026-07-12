package services

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"time"
)

// HolderIntelligenceRow is the human-facing owner-level holding surface. Token
// accounts controlled by the same resolved owner are aggregated. Unresolved
// token accounts remain separate and are never silently attributed to a wallet.
type HolderIntelligenceRow struct {
	Rank                     int      `json:"rank"`
	OwnerWallet              string   `json:"owner_wallet,omitempty"`
	OwnerResolved            bool     `json:"owner_resolved"`
	TokenAccounts            []string `json:"token_accounts"`
	TokenAccountCount        int      `json:"token_account_count"`
	Role                     string   `json:"role"`
	RoleConfidence           string   `json:"role_confidence"`
	RiskBearing              bool     `json:"risk_bearing"`
	ExcludedFromHolderRisk   bool     `json:"excluded_from_holder_risk"`
	Balance                  float64  `json:"balance"`
	RawPercentage            float64  `json:"raw_percentage"`
	CirculatingPercentage    float64  `json:"circulating_percentage,omitempty"`
	ReferenceUSDValue        *float64 `json:"reference_usd_value,omitempty"`
	AcquisitionObserved      bool     `json:"acquisition_observed"`
	AcquisitionObservedAt    string   `json:"acquisition_observed_at,omitempty"`
	OldestActivityObservedAt string   `json:"oldest_activity_observed_at,omitempty"`
	NewestActivityObservedAt string   `json:"newest_activity_observed_at,omitempty"`
	ObservedHoldingDays      int      `json:"observed_holding_days,omitempty"`
	ObservedActivityAgeDays  int      `json:"observed_activity_age_days,omitempty"`
	HoldingDurationScope     string   `json:"holding_duration_scope"`
	HistoryExhausted         bool     `json:"history_exhausted"`
	SignaturesObserved       int      `json:"signatures_observed"`
	ParsedTransactions       int      `json:"parsed_transactions"`
	OutflowTransactions      int      `json:"outflow_transactions"`
	CommonExitObserved       bool     `json:"common_exit_observed"`
	CommonExitRecipient      string   `json:"common_exit_recipient,omitempty"`
	FreshNearLaunch          bool     `json:"fresh_near_launch"`
	FundingSource            string   `json:"funding_source,omitempty"`
	Behavior                 string   `json:"behavior"`
	Evidence                 []string `json:"evidence"`
}

// HolderIntelligence keeps factual holdings visible even when an unresolved
// dominant role correctly blocks a final risk verdict.
type HolderIntelligence struct {
	Available                  bool                    `json:"available"`
	Status                     string                  `json:"status"`
	FinalVerdictBlocked        bool                    `json:"final_verdict_blocked"`
	OwnerAggregationApplied    bool                    `json:"owner_aggregation_applied"`
	Supply                     float64                 `json:"supply"`
	CirculatingSupply          float64                 `json:"circulating_supply"`
	PriceUSD                   float64                 `json:"price_usd"`
	Market                     TokenMarketSnapshot     `json:"market"`
	OwnerCount                 int                     `json:"owner_count"`
	RiskBearingOwnerCount      int                     `json:"risk_bearing_owner_count"`
	ProtocolOwnerCount         int                     `json:"protocol_owner_count"`
	UnresolvedOwnerCount       int                     `json:"unresolved_owner_count"`
	WalletsWithParsedEvidence  int                     `json:"wallets_with_parsed_evidence"`
	WalletsWithObservedInflow  int                     `json:"wallets_with_observed_inflow"`
	WalletsWithObservedOutflow int                     `json:"wallets_with_observed_outflow"`
	CommonExitGroupCount       int                     `json:"common_exit_group_count"`
	Top1Percentage             float64                 `json:"top_1_percentage"`
	Top3Percentage             float64                 `json:"top_3_percentage"`
	Top10Percentage            float64                 `json:"top_10_percentage"`
	Top20Percentage            float64                 `json:"top_20_percentage"`
	TopOwnerBalance            float64                 `json:"top_owner_balance"`
	TopOwnerPercentage         float64                 `json:"top_owner_percentage"`
	TopOwnerReferenceUSDValue  *float64                `json:"top_owner_reference_usd_value,omitempty"`
	Rows                       []HolderIntelligenceRow `json:"rows"`
	Findings                   []string                `json:"findings"`
	Limitations                []string                `json:"limitations"`
	GeneratedAt                string                  `json:"generated_at"`
}

type holderOwnerAggregate struct {
	OwnerWallet    string
	OwnerResolved  bool
	TokenAccounts  []string
	Role           string
	RoleConfidence string
	Excluded       bool
	Balance        float64
	Evidence       []string
	mixedRole      bool
}

func BuildHolderIntelligence(roles HolderRoleAnalysis, cluster HolderClusterAnalysis, market TokenMarketSnapshot, now time.Time) HolderIntelligence {
	if now.IsZero() {
		now = time.Now().UTC()
	}
	out := HolderIntelligence{
		Status: "holder_data_unavailable", Supply: roles.Supply, CirculatingSupply: roles.CirculatingSupply,
		PriceUSD: market.PriceUSD, Market: market, Rows: []HolderIntelligenceRow{}, Findings: []string{},
		Limitations: []string{}, GeneratedAt: now.UTC().Format(time.RFC3339), FinalVerdictBlocked: roles.BlockingEvidenceGap,
	}
	if !roles.Available || len(roles.Accounts) == 0 || roles.Supply <= 0 {
		out.Limitations = append(out.Limitations, "Token supply and resolved largest-account evidence are required for holder intelligence.")
		out.Limitations = append(out.Limitations, market.Limitations...)
		return out
	}

	aggregates := map[string]*holderOwnerAggregate{}
	order := []string{}
	for _, account := range roles.Accounts {
		owner := strings.TrimSpace(account.OwnerWallet)
		resolved := owner != ""
		key := owner
		if key == "" {
			key = "token-account:" + strings.TrimSpace(account.TokenAccount)
		}
		agg := aggregates[key]
		if agg == nil {
			agg = &holderOwnerAggregate{
				OwnerWallet: owner, OwnerResolved: resolved, Role: account.Role,
				RoleConfidence: account.Confidence, Excluded: account.ExcludedFromHolderRisk,
				TokenAccounts: []string{}, Evidence: []string{},
			}
			aggregates[key] = agg
			order = append(order, key)
		}
		if agg.Role != account.Role {
			agg.mixedRole = true
			agg.Role = "mixed_control_surface"
			agg.RoleConfidence = "medium"
		}
		if !account.ExcludedFromHolderRisk {
			agg.Excluded = false
		}
		agg.Balance += account.Balance
		agg.TokenAccounts = append(agg.TokenAccounts, account.TokenAccount)
		agg.Evidence = appendUniqueHolderEvidence(agg.Evidence, account.Evidence...)
	}

	rows := make([]HolderIntelligenceRow, 0, len(order))
	for _, key := range order {
		agg := aggregates[key]
		row := HolderIntelligenceRow{
			OwnerWallet: agg.OwnerWallet, OwnerResolved: agg.OwnerResolved,
			TokenAccounts: append([]string{}, agg.TokenAccounts...), TokenAccountCount: len(agg.TokenAccounts),
			Role: agg.Role, RoleConfidence: agg.RoleConfidence, ExcludedFromHolderRisk: agg.Excluded,
			RiskBearing: !agg.Excluded, Balance: roundHolderIntelligence(agg.Balance, 8),
			RawPercentage:        roundHolderIntelligence(holderIntelligencePercent(agg.Balance, roles.Supply), 4),
			HoldingDurationScope: "not_observed", Behavior: holderIntelligenceBaseBehavior(agg),
			Evidence: append([]string{}, agg.Evidence...),
		}
		if !agg.Excluded && roles.CirculatingSupply > 0 {
			row.CirculatingPercentage = roundHolderIntelligence(holderIntelligencePercent(agg.Balance, roles.CirculatingSupply), 4)
		}
		if market.PriceUSD > 0 {
			value := roundHolderIntelligence(agg.Balance*market.PriceUSD, 2)
			row.ReferenceUSDValue = &value
		}
		rows = append(rows, row)
	}
	sort.SliceStable(rows, func(i, j int) bool { return rows[i].Balance > rows[j].Balance })
	for i := range rows {
		rows[i].Rank = i + 1
	}

	clusterByWallet := map[string]HolderClusterWallet{}
	for _, wallet := range cluster.Wallets {
		clusterByWallet[strings.TrimSpace(wallet.Wallet)] = wallet
	}
	commonExitByWallet := map[string]string{}
	for _, group := range cluster.Flow.CommonExitGroups {
		for _, wallet := range group.Wallets {
			commonExitByWallet[strings.TrimSpace(wallet)] = group.Key
		}
	}
	for i := range rows {
		wallet := strings.TrimSpace(rows[i].OwnerWallet)
		observed, ok := clusterByWallet[wallet]
		if !ok || wallet == "" {
			continue
		}
		rows[i].HistoryExhausted = observed.HistoryExhausted
		rows[i].SignaturesObserved = observed.SignaturesObserved
		rows[i].ParsedTransactions = observed.ParsedTransactions
		rows[i].AcquisitionObservedAt = observed.AcquisitionObservedAt
		rows[i].AcquisitionObserved = observed.AcquisitionSlot > 0 || strings.TrimSpace(observed.AcquisitionObservedAt) != ""
		rows[i].OldestActivityObservedAt = observed.OldestObservedAt
		rows[i].NewestActivityObservedAt = observed.NewestObservedAt
		rows[i].FreshNearLaunch = observed.FreshNearLaunch
		rows[i].FundingSource = observed.FundingSource
		rows[i].OutflowTransactions = holderIntelligenceUniqueOutflows(observed.FlowObservations)
		if recipient := strings.TrimSpace(commonExitByWallet[wallet]); recipient != "" {
			rows[i].CommonExitObserved = true
			rows[i].CommonExitRecipient = recipient
		}
		if rows[i].AcquisitionObserved {
			if parsed, err := time.Parse(time.RFC3339, rows[i].AcquisitionObservedAt); err == nil {
				rows[i].ObservedHoldingDays = holderIntelligenceAgeDays(now, parsed)
				rows[i].HoldingDurationScope = "bounded_first_observed_acquisition"
			}
		}
		if parsed, err := time.Parse(time.RFC3339, rows[i].OldestActivityObservedAt); err == nil {
			rows[i].ObservedActivityAgeDays = holderIntelligenceAgeDays(now, parsed)
			if rows[i].HoldingDurationScope == "not_observed" {
				rows[i].HoldingDurationScope = "wallet_activity_only_not_token_holding"
			}
		}
		rows[i].Behavior = holderIntelligenceObservedBehavior(rows[i])
		rows[i].Evidence = appendUniqueHolderEvidence(rows[i].Evidence,
			fmt.Sprintf("Bounded wallet history observed %d signatures and parsed %d transactions.", rows[i].SignaturesObserved, rows[i].ParsedTransactions),
		)
	}

	out.Available = true
	out.Status = "verified_holdings_final_pending"
	if !roles.BlockingEvidenceGap {
		out.Status = "verified_owner_aggregated_holdings"
	}
	out.OwnerAggregationApplied = true
	out.Rows = rows
	out.OwnerCount = len(rows)
	out.CommonExitGroupCount = cluster.Flow.CommonExitGroupCount
	for _, row := range rows {
		switch {
		case row.ExcludedFromHolderRisk:
			out.ProtocolOwnerCount++
		case !row.OwnerResolved || strings.Contains(row.Role, "unresolved") || row.Role == "wallet_account_unavailable":
			out.UnresolvedOwnerCount++
			out.RiskBearingOwnerCount++
		default:
			out.RiskBearingOwnerCount++
		}
		if row.ParsedTransactions > 0 {
			out.WalletsWithParsedEvidence++
		}
		if row.AcquisitionObserved {
			out.WalletsWithObservedInflow++
		}
		if row.OutflowTransactions > 0 {
			out.WalletsWithObservedOutflow++
		}
	}
	out.Top1Percentage, out.Top3Percentage, out.Top10Percentage, out.Top20Percentage = holderIntelligenceConcentration(rows, roles.CirculatingSupply)
	if top := holderIntelligenceTopRiskRow(rows); top != nil {
		out.TopOwnerBalance = top.Balance
		out.TopOwnerPercentage = top.RawPercentage
		if top.CirculatingPercentage > 0 {
			out.TopOwnerPercentage = top.CirculatingPercentage
		}
		out.TopOwnerReferenceUSDValue = top.ReferenceUSDValue
	}
	out.Findings = holderIntelligenceFindings(out)
	out.Limitations = append(out.Limitations, roles.Limitations...)
	out.Limitations = append(out.Limitations, cluster.Limitations...)
	out.Limitations = append(out.Limitations, market.Limitations...)
	out.Limitations = appendUniqueHolderEvidence([]string{}, out.Limitations...)
	return out
}

func holderIntelligenceTopRiskRow(rows []HolderIntelligenceRow) *HolderIntelligenceRow {
	for i := range rows {
		if rows[i].RiskBearing {
			return &rows[i]
		}
	}
	return nil
}

func holderIntelligenceConcentration(rows []HolderIntelligenceRow, circulatingSupply float64) (float64, float64, float64, float64) {
	if circulatingSupply <= 0 {
		return 0, 0, 0, 0
	}
	values := []float64{}
	for _, row := range rows {
		if row.RiskBearing {
			values = append(values, row.Balance)
		}
	}
	var top1, top3, top10, top20 float64
	for i, value := range values {
		pct := holderIntelligencePercent(value, circulatingSupply)
		if i == 0 {
			top1 = pct
		}
		if i < 3 {
			top3 += pct
		}
		if i < 10 {
			top10 += pct
		}
		if i < 20 {
			top20 += pct
		}
	}
	return roundHolderIntelligence(top1, 4), roundHolderIntelligence(top3, 4), roundHolderIntelligence(top10, 4), roundHolderIntelligence(top20, 4)
}

func holderIntelligenceFindings(out HolderIntelligence) []string {
	findings := []string{}
	if topRow := holderIntelligenceTopRiskRow(out.Rows); topRow != nil {
		top := *topRow
		ownerLabel := top.OwnerWallet
		if ownerLabel == "" && len(top.TokenAccounts) > 0 {
			ownerLabel = top.TokenAccounts[0]
		}
		valueText := ""
		if top.ReferenceUSDValue != nil {
			valueText = fmt.Sprintf("; referans piyasa değeri yaklaşık $%.2f", *top.ReferenceUSDValue)
		}
		findings = append(findings, fmt.Sprintf("En büyük gözlenen kontrol yüzeyi %s üzerinde %.4f token ve arzın yaklaşık %.4f%%'sidir%s.", shortHolderIntelligence(ownerLabel), top.Balance, out.TopOwnerPercentage, valueText))
		if !top.OwnerResolved || strings.Contains(top.Role, "unresolved") || top.Role == "wallet_account_unavailable" {
			findings = append(findings, "Baskın hesabın owner/ekonomik rolü kesin çözülemediği için bu miktar saklanmaz veya LOW sayılmaz; final karar kanıt tamamlanana kadar bekletilir.")
		}
	}
	if out.PriceUSD > 0 {
		findings = append(findings, fmt.Sprintf("Referans fiyat $%.10f; 24 saatlik gözlenen hacim $%.2f ve toplam likidite $%.2f.", out.PriceUSD, out.Market.Volume24hUSD, out.Market.LiquidityUSD))
	}
	if out.WalletsWithObservedOutflow > 0 {
		findings = append(findings, fmt.Sprintf("Sınırlı işlem penceresinde %d holder wallet için hedef-token çıkışı gözlendi.", out.WalletsWithObservedOutflow))
	}
	if out.CommonExitGroupCount > 0 {
		findings = append(findings, fmt.Sprintf("%d ortak recipient-owner çıkış grubu gözlendi; bu ilişki tek başına ortak sahiplik veya satış kanıtı değildir.", out.CommonExitGroupCount))
	}
	if out.WalletsWithParsedEvidence == 0 {
		findings = append(findings, "Holder bakiyeleri doğrulandı ancak cüzdan davranışı için parsed transaction kanıtı üretilemedi.")
	}
	return findings
}

func holderIntelligenceBaseBehavior(agg *holderOwnerAggregate) string {
	if agg.Excluded {
		return "protocol_or_non_wallet_inventory"
	}
	if !agg.OwnerResolved || strings.Contains(agg.Role, "unresolved") || agg.Role == "wallet_account_unavailable" {
		return "owner_role_unresolved"
	}
	return "holding_observed_no_behavior_window"
}

func holderIntelligenceObservedBehavior(row HolderIntelligenceRow) string {
	switch {
	case row.CommonExitObserved:
		return "common_exit_recipient_observed"
	case row.OutflowTransactions > 0:
		return "token_outflow_observed"
	case row.AcquisitionObserved:
		return "acquisition_observed_no_outflow_in_bounded_window"
	case row.ParsedTransactions > 0:
		return "wallet_observed_no_target_token_flow"
	default:
		return row.Behavior
	}
}

func holderIntelligenceUniqueOutflows(values []HolderClusterFlowObservation) int {
	seen := map[string]bool{}
	for _, value := range values {
		if value.Amount <= 0 || strings.TrimSpace(value.SourceWallet) == "" {
			continue
		}
		key := strings.TrimSpace(value.Signature)
		if key == "" {
			key = value.SourceWallet + "|" + value.Destination + "|" + fmt.Sprintf("%.9f", value.Amount)
		}
		seen[key] = true
	}
	return len(seen)
}

func holderIntelligenceAgeDays(now, observed time.Time) int {
	if observed.IsZero() || now.Before(observed) {
		return 0
	}
	return int(now.Sub(observed).Hours() / 24)
}

func holderIntelligencePercent(value, denominator float64) float64 {
	if value <= 0 || denominator <= 0 {
		return 0
	}
	return value / denominator * 100
}

func roundHolderIntelligence(value float64, digits int) float64 {
	factor := math.Pow10(digits)
	return math.Round(value*factor) / factor
}

func appendUniqueHolderEvidence(dst []string, values ...string) []string {
	seen := map[string]bool{}
	for _, value := range dst {
		seen[value] = true
	}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		dst = append(dst, value)
	}
	return dst
}

func shortHolderIntelligence(value string) string {
	value = strings.TrimSpace(value)
	if len(value) <= 18 {
		return value
	}
	return value[:8] + "…" + value[len(value)-6:]
}
