from pathlib import Path


def replace_once(text: str, old: str, new: str, label: str) -> str:
    if old not in text:
        raise SystemExit(f"missing replacement target: {label}")
    return text.replace(old, new, 1)

# Price selection: any positive-liquidity base pair outranks a no-liquidity pair,
# regardless of reported volume. This prevents a thin/no-liquidity market from
# becoming the holder USD reference merely because volume is large.
path = Path("internal/services/token_market_snapshot.go")
text = path.read_text()
text = replace_once(
    text,
    '''\tseen := map[string]bool{}
\tbestMetric := -1.0''',
    '''\tseen := map[string]bool{}
\tbestHasLiquidity := false
\tbestLiquidity := -1.0
\tbestVolume := -1.0''',
    "market best-pair state",
)
text = replace_once(
    text,
    '''\t\tmetric := liquidity
\t\tif metric <= 0 {
\t\t\tmetric = volume / 1000000
\t\t}
\t\tif metric < bestMetric {
\t\t\tcontinue
\t\t}
\t\tprice, _ := strconv.ParseFloat(strings.TrimSpace(pair.PriceUSD), 64)''',
    '''\t\tbetter := false
\t\tif liquidity > 0 {
\t\t\tbetter = !bestHasLiquidity || liquidity > bestLiquidity
\t\t} else if !bestHasLiquidity {
\t\t\tbetter = volume > bestVolume
\t\t}
\t\tif !better {
\t\t\tcontinue
\t\t}
\t\tprice, _ := strconv.ParseFloat(strings.TrimSpace(pair.PriceUSD), 64)''',
    "market liquid pair selection",
)
text = replace_once(
    text,
    '''\t\tbestMetric = metric
\t\tout.Name = strings.TrimSpace(pair.BaseToken.Name)''',
    '''\t\tbestHasLiquidity = liquidity > 0
\t\tbestLiquidity = liquidity
\t\tbestVolume = volume
\t\tout.Name = strings.TrimSpace(pair.BaseToken.Name)''',
    "market best-pair state update",
)
path.write_text(text)

# Top-owner summary must describe the largest risk-bearing owner, not a protocol
# vault that is intentionally excluded from holder concentration.
path = Path("internal/services/holder_intelligence.go")
text = path.read_text()
text = replace_once(
    text,
    '''\tif len(rows) > 0 {
\t\tout.TopOwnerBalance = rows[0].Balance
\t\tout.TopOwnerPercentage = rows[0].RawPercentage
\t\tif rows[0].RiskBearing && rows[0].CirculatingPercentage > 0 {
\t\t\tout.TopOwnerPercentage = rows[0].CirculatingPercentage
\t\t}
\t\tout.TopOwnerReferenceUSDValue = rows[0].ReferenceUSDValue
\t}''',
    '''\tif top := holderIntelligenceTopRiskRow(rows); top != nil {
\t\tout.TopOwnerBalance = top.Balance
\t\tout.TopOwnerPercentage = top.RawPercentage
\t\tif top.CirculatingPercentage > 0 {
\t\t\tout.TopOwnerPercentage = top.CirculatingPercentage
\t\t}
\t\tout.TopOwnerReferenceUSDValue = top.ReferenceUSDValue
\t}''',
    "risk-bearing top summary",
)
text = replace_once(
    text,
    '''\tif len(out.Rows) > 0 {
\t\ttop := out.Rows[0]
\t\townerLabel := top.OwnerWallet''',
    '''\tif topRow := holderIntelligenceTopRiskRow(out.Rows); topRow != nil {
\t\ttop := *topRow
\t\townerLabel := top.OwnerWallet''',
    "risk-bearing top finding",
)
anchor = '''func holderIntelligenceConcentration(rows []HolderIntelligenceRow, circulatingSupply float64) (float64, float64, float64, float64) {'''
helper = '''func holderIntelligenceTopRiskRow(rows []HolderIntelligenceRow) *HolderIntelligenceRow {
\tfor i := range rows {
\t\tif rows[i].RiskBearing {
\t\t\treturn &rows[i]
\t\t}
\t}
\treturn nil
}

'''
text = replace_once(text, anchor, helper + anchor, "top risk row helper")
path.write_text(text)

# Reuse holder roles already collected inside the full ARVIS run. Falling back to
# a second RPC pass remains available only if typed evidence is missing.
path = Path("internal/services/arvis_arms.go")
text = path.read_text()
anchor = '''func ArvisHolderClusterFromBundle(bundle SecurityRadarBundle) HolderClusterAnalysis {'''
helper = '''func ArvisHolderRolesFromBundle(bundle SecurityRadarBundle) HolderRoleAnalysis {
\tfor _, arm := range ArvisArmsFromBundle(bundle) {
\t\tif arm.ModuleID != ModuleHolderConcentration || arm.Signals == nil {
\t\t\tcontinue
\t\t}
\t\tif value, ok := arm.Signals["holder_role_analysis"].(HolderRoleAnalysis); ok {
\t\t\treturn value
\t\t}
\t}
\treturn HolderRoleAnalysis{}
}

'''
text = replace_once(text, anchor, helper + anchor, "holder roles bundle helper")
path.write_text(text)

path = Path("internal/handlers/security_radar_detail.go")
text = path.read_text()
text = replace_once(
    text,
    '''\tdistribution, holderRoles := radarDetailHolderDistribution(r.Context(), target)
\tholderCluster := services.ArvisHolderClusterFromBundle(bundle)''',
    '''\tholderRoles := services.ArvisHolderRolesFromBundle(bundle)
\tdistribution := radarDetailHolderDistributionFromRoles(holderRoles)
\tif !holderRoles.Available {
\t\tdistribution, holderRoles = radarDetailHolderDistribution(r.Context(), target)
\t}
\tholderCluster := services.ArvisHolderClusterFromBundle(bundle)''',
    "premium reuse holder roles",
)
anchor = '''func radarDetailHolderDistribution(parent context.Context, target string) (map[string]any, services.HolderRoleAnalysis) {'''
helper = '''func radarDetailHolderDistributionFromRoles(roles services.HolderRoleAnalysis) map[string]any {
\tif !roles.Available {
\t\treturn map[string]any{"available": false, "status": "holder_roles_unavailable", "top_accounts": []any{}}
\t}
\tout := services.HolderRoleAnalysisMap(roles)
\tout["largest_account_balance"] = 0.0
\tif len(roles.Accounts) > 0 {
\t\tout["largest_account_balance"] = radarDetailRound(roles.Accounts[0].Balance, 6)
\t}
\tout["account_scope"] = "Token accounts resolved to owner wallets and owner programs; only positively identified protocol inventory or burn sinks are excluded from holder-risk concentration."
\tout["evidence_reused_from_full_scan"] = true
\treturn out
}

'''
text = replace_once(text, anchor, helper + anchor, "holder distribution from bundle")
path.write_text(text)

path = Path("internal/handlers/owner_operations.go")
text = path.read_text()
text = replace_once(
    text,
    '''\tdistribution, holderRoles := radarDetailHolderDistribution(r.Context(), target)
\tholderCluster := services.ArvisHolderClusterFromBundle(bundle)''',
    '''\tholderRoles := services.ArvisHolderRolesFromBundle(bundle)
\tdistribution := radarDetailHolderDistributionFromRoles(holderRoles)
\tif !holderRoles.Available {
\t\tdistribution, holderRoles = radarDetailHolderDistribution(r.Context(), target)
\t}
\tholderCluster := services.ArvisHolderClusterFromBundle(bundle)''',
    "owner reuse holder roles",
)
path.write_text(text)

# The badge itself must be human-readable Turkish, not a raw machine enum.
path = Path("public/js/owner-control-center.js")
text = path.read_text()
text = replace_once(
    text,
    '''<td>${badge(r.behavior)}<div class="muted small">${esc(holderBehaviorLabel(r.behavior))}${r.outflow_transactions?` · ${num(r.outflow_transactions)} çıkış`:''}</div></td>''',
    '''<td><span class="badge warn">${esc(holderBehaviorLabel(r.behavior))}</span><div class="muted small">${r.outflow_transactions?`${num(r.outflow_transactions)} doğrulanmış çıkış`:'Sınırlı gözlem penceresi'}</div></td>''',
    "human behavior badge",
)
path.write_text(text)

# Tests for the two hard rules.
path = Path("internal/services/holder_intelligence_test.go")
text = path.read_text()
text += '''

func TestHolderIntelligenceTopSummaryExcludesProtocolInventory(t *testing.T) {
\troles := HolderRoleAnalysis{
\t\tAvailable: true, Supply: 1000, CirculatingSupply: 100,
\t\tAccounts: []HolderRoleAccount{
\t\t\t{Rank: 1, TokenAccount: "Protocol", OwnerWallet: "ProtocolPDA", Balance: 900, Role: "pump_liquidity_vault", Confidence: "high", ExcludedFromHolderRisk: true},
\t\t\t{Rank: 2, TokenAccount: "Wallet", OwnerWallet: "WalletA", Balance: 100, Role: "externally_owned_wallet", Confidence: "high", ExcludedFromHolderRisk: false},
\t\t},
\t}
\tresult := BuildHolderIntelligence(roles, HolderClusterAnalysis{}, TokenMarketSnapshot{PriceUSD: 2}, time.Now().UTC())
\tif result.TopOwnerBalance != 100 || result.TopOwnerPercentage != 100 {
\t\tt.Fatalf("protocol inventory became top risk-bearing owner: %#v", result)
\t}
\tif result.TopOwnerReferenceUSDValue == nil || *result.TopOwnerReferenceUSDValue != 200 {
\t\tt.Fatalf("top owner value = %#v", result.TopOwnerReferenceUSDValue)
\t}
}
'''
path.write_text(text)

path = Path("internal/services/token_market_snapshot_test.go")
text = path.read_text()
text += '''

func TestTokenMarketSnapshotPositiveLiquidityOutranksNoLiquidityVolume(t *testing.T) {
\tmint := "MintLiquidity11111111111111111111111111111111"
\tserver := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
\t\tw.Header().Set("Content-Type", "application/json")
\t\t_, _ = w.Write([]byte(`[
\t\t\t{"chainId":"solana","dexId":"no-liquidity","pairAddress":"a","baseToken":{"address":"` + mint + `"},"quoteToken":{"address":"SOL"},"priceUsd":"99","volume":{"h24":900000000}},
\t\t\t{"chainId":"solana","dexId":"liquid","pairAddress":"b","baseToken":{"address":"` + mint + `"},"quoteToken":{"address":"USDC"},"priceUsd":"0.10","volume":{"h24":1000},"liquidity":{"usd":10}}
\t\t]`))
\t}))
\tdefer server.Close()
\tmarket := (&TokenMarketClient{Endpoint: server.URL, Client: server.Client()}).Fetch(context.Background(), mint)
\tif market.PriceUSD != 0.10 || market.BestPairAddress != "b" {
\t\tt.Fatalf("no-liquidity pair became price reference: %#v", market)
\t}
}
'''
path.write_text(text)
