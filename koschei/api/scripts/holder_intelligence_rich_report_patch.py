from pathlib import Path
import re


def replace_once(text: str, old: str, new: str, label: str) -> str:
    if old not in text:
        raise SystemExit(f"missing replacement target: {label}")
    return text.replace(old, new, 1)

# Typed extraction of the cluster analysis already collected during ARVIS.
path = Path("internal/services/arvis_arms.go")
text = path.read_text()
anchor = '''func arvisSourceModule(mode string) string {'''
helper = '''func ArvisHolderClusterFromBundle(bundle SecurityRadarBundle) HolderClusterAnalysis {
	if bundle.Metadata == nil {
		return HolderClusterAnalysis{}
	}
	if value, ok := bundle.Metadata["holder_cluster_analysis"].(HolderClusterAnalysis); ok {
		return value
	}
	return HolderClusterAnalysis{}
}

'''
text = replace_once(text, anchor, helper + anchor, "cluster bundle helper")
path.write_text(text)

# Premium detail contract: expose market and owner-level holdings even if final is pending.
path = Path("internal/handlers/security_radar_detail.go")
text = path.read_text()
text = replace_once(
    text,
    '''\tdistribution := radarDetailHolderDistribution(r.Context(), target)
\tsourceContext := h.radarDetailSourceContext(r.Context(), target, network)''',
    '''\tdistribution, holderRoles := radarDetailHolderDistribution(r.Context(), target)
\tholderCluster := services.ArvisHolderClusterFromBundle(bundle)
\tmarket := radarDetailMarketSnapshot(r.Context(), target)
\tholderIntelligence := services.BuildHolderIntelligence(holderRoles, holderCluster, market, time.Now().UTC())
\tsourceContext := h.radarDetailSourceContext(r.Context(), target, network)''',
    "premium holder intelligence collection",
)
text = replace_once(
    text,
    '''\t\t"holder_distribution": distribution,
\t\t"structural_memory":   structural,''',
    '''\t\t"holder_distribution": distribution,
\t\t"holder_intelligence": holderIntelligence,
\t\t"holder_cluster":      holderCluster,
\t\t"market":              market,
\t\t"structural_memory":   structural,''',
    "premium holder intelligence response",
)
text = replace_once(
    text,
    '''func radarDetailHolderDistribution(parent context.Context, target string) map[string]any {''',
    '''func radarDetailHolderDistribution(parent context.Context, target string) (map[string]any, services.HolderRoleAnalysis) {''',
    "holder distribution signature",
)
text = text.replace(
    '''return map[string]any{"available": false, "status": "rpc_not_configured", "top_accounts": []any{}}''',
    '''return map[string]any{"available": false, "status": "rpc_not_configured", "top_accounts": []any{}}, services.HolderRoleAnalysis{}''',
    1,
)
text = text.replace(
    '''return map[string]any{"available": false, "status": "supply_unavailable", "error": compactRadarDetailError(err), "top_accounts": []any{}}''',
    '''return map[string]any{"available": false, "status": "supply_unavailable", "error": compactRadarDetailError(err), "top_accounts": []any{}}, services.HolderRoleAnalysis{}''',
    1,
)
text = text.replace(
    '''return map[string]any{"available": false, "status": "largest_accounts_unavailable", "error": compactRadarDetailError(err), "top_accounts": []any{}}''',
    '''return map[string]any{"available": false, "status": "largest_accounts_unavailable", "error": compactRadarDetailError(err), "top_accounts": []any{}}, services.HolderRoleAnalysis{}''',
    1,
)
text = replace_once(text, '''\treturn out
}

func radarDetailTokenAmount''', '''\treturn out, roles
}

func radarDetailMarketSnapshot(parent context.Context, target string) services.TokenMarketSnapshot {
\tctx, cancel := context.WithTimeout(parent, 6*time.Second)
\tdefer cancel()
\treturn services.FetchSolanaTokenMarketSnapshot(ctx, target)
}

func radarDetailTokenAmount''', "holder distribution typed return")
path.write_text(text)

# Owner full scan: factual explanation survives unsigned final verdicts.
path = Path("internal/handlers/owner_operations.go")
text = path.read_text()
text = replace_once(
    text,
    '''\tdistribution := radarDetailHolderDistribution(r.Context(), target)
\tsourceContext := h.radarDetailSourceContext(r.Context(), target, network)''',
    '''\tdistribution, holderRoles := radarDetailHolderDistribution(r.Context(), target)
\tholderCluster := services.ArvisHolderClusterFromBundle(bundle)
\tmarket := radarDetailMarketSnapshot(r.Context(), target)
\tholderIntelligence := services.BuildHolderIntelligence(holderRoles, holderCluster, market, time.Now().UTC())
\tsourceContext := h.radarDetailSourceContext(r.Context(), target, network)''',
    "owner holder intelligence collection",
)
text = replace_once(
    text,
    '''\t\t"final_verdict": final, "warning": warning, "holder_distribution": distribution,
\t\t"structural_memory": structural, "source_context": sourceContext,
\t\t"modules": modules, "evidence": evidence, "graph": graph,
\t\t"holder_cluster": ownerRadarModuleSignal(modules, services.ModuleFundingClusterDetector, "holder_cluster_analysis"),''',
    '''\t\t"final_verdict": final, "warning": warning, "holder_distribution": distribution,
\t\t"holder_intelligence": holderIntelligence, "holder_cluster": holderCluster, "market": market,
\t\t"structural_memory": structural, "source_context": sourceContext,
\t\t"modules": modules, "evidence": evidence, "graph": graph,''',
    "owner holder intelligence response",
)
text = replace_once(
    text,
    '''\tdetail["narrative"] = ownerRadarNarrative(target, final, warning, distribution, sourceContext, modules)''',
    '''\tdetail["narrative"] = ownerRadarNarrative(target, final, warning, distribution, sourceContext, modules, holderIntelligence)''',
    "owner narrative input",
)
text = replace_once(
    text,
    '''func ownerRadarNarrative(target string, final, warning, distribution, source map[string]any, modules []map[string]any) string {''',
    '''func ownerRadarNarrative(target string, final, warning, distribution, source map[string]any, modules []map[string]any, holder services.HolderIntelligence) string {''',
    "owner narrative signature",
)
text = replace_once(
    text,
    '''\tif !signed || final["risk_index"] == nil || level == "" || level == "unknown" || level == "<nil>" {
\t\treturn "Koschei bu hedef için doğrulanmış bir final risk puanı üretmedi. Kanıt eksikliği düşük risk anlamına gelmez; eksik modüller tamamlanana kadar sonuç EVIDENCE PENDING olarak değerlendirilmelidir."
\t}''',
    '''\tif !signed || final["risk_index"] == nil || level == "" || level == "unknown" || level == "<nil>" {
\t\treturn ownerRadarPendingNarrative(target, holder, distribution, warning)
\t}''',
    "pending factual narrative",
)
text = replace_once(
    text,
    '''\tparts = append(parts, ownerRadarPracticalConclusion(level, distribution))''',
    '''\tif holder.Available {
\t\tfor _, finding := range holder.Findings {
\t\t\tif strings.TrimSpace(finding) != "" {
\t\t\t\tparts = append(parts, finding)
\t\t\t}
\t\t}
\t}
\tparts = append(parts, ownerRadarPracticalConclusion(level, distribution))''',
    "signed holder findings",
)
insert_anchor = '''func ownerRadarPrimaryRiskDriver(modules []map[string]any) map[string]any {'''
pending_helper = '''func ownerRadarPendingNarrative(target string, holder services.HolderIntelligence, distribution, warning map[string]any) string {
\tparts := []string{
\t\t"Koschei bu hedef için doğrulanmış bir final risk puanı üretmedi; bu, elde veri olmadığı veya tokenın güvenli olduğu anlamına gelmez.",
\t}
\tif holder.Available {
\t\tparts = append(parts, fmt.Sprintf("Zincir üstü holder yüzeyi yine doğrulandı: toplam arz %.4f token, owner bazında %d kontrol yüzeyi ve %d risk taşıyan owner gözlendi.", holder.Supply, holder.OwnerCount, holder.RiskBearingOwnerCount))
\t\tif len(holder.Rows) > 0 {
\t\t\ttop := holder.Rows[0]
\t\t\towner := top.OwnerWallet
\t\t\tif owner == "" && len(top.TokenAccounts) > 0 {
\t\t\t\towner = top.TokenAccounts[0]
\t\t\t}
\t\t\tusd := ""
\t\t\tif top.ReferenceUSDValue != nil {
\t\t\t\tusd = fmt.Sprintf("; referans piyasa değeri yaklaşık $%.2f", *top.ReferenceUSDValue)
\t\t\t}
\t\t\tparts = append(parts, fmt.Sprintf("En büyük gözlenen hesap %s üzerinde %.4f token tutuyor ve owner-bazlı arz payı yaklaşık %.4f%%%s.", ownerRadarShortTarget(owner), top.Balance, holder.TopOwnerPercentage, usd))
\t\t\tif !top.OwnerResolved || strings.Contains(top.Role, "unresolved") || top.Role == "wallet_account_unavailable" {
\t\t\t\tparts = append(parts, "Bu baskın hesabın owner veya ekonomik rolü çözülemediği için Koschei yüksek yoğunluğu saklamadı fakat wallet-control finali de uydurmadı; sonuç bu nedenle EVIDENCE PENDING kaldı.")
\t\t\t}
\t\t}
\t\tif holder.Market.Available {
\t\t\tparts = append(parts, fmt.Sprintf("Piyasa bağlamı: referans fiyat $%.10f, 24 saatlik hacim $%.2f, likidite $%.2f ve piyasa değeri $%.2f.", holder.Market.PriceUSD, holder.Market.Volume24hUSD, holder.Market.LiquidityUSD, holder.Market.MarketCapUSD))
\t\t}
\t\tif holder.WalletsWithObservedOutflow > 0 {
\t\t\tparts = append(parts, fmt.Sprintf("Sınırlı cüzdan geçmişinde %d holder wallet için hedef-token çıkışı gözlendi.", holder.WalletsWithObservedOutflow))
\t\t}
\t\tif holder.CommonExitGroupCount > 0 {
\t\t\tparts = append(parts, fmt.Sprintf("%d ortak recipient-owner çıkış grubu doğrulandı; bu ilişki tek başına ortak sahiplik, satış veya wash trading kanıtı değildir.", holder.CommonExitGroupCount))
\t\t}
\t}
\tfor _, reason := range ownerRadarStringSlice(warning["reasons"]) {
\t\tif strings.TrimSpace(reason) != "" {
\t\t\tparts = append(parts, reason)
\t\t}
\t}
\tparts = append(parts, "Final skor için eksik rol ve modül kanıtları tamamlanmalıdır; mevcut gerçek bakiyeler, yüzdeler ve davranış gözlemleri aşağıdaki holder tablosunda gösterilir.")
\treturn strings.Join(parts, " ")
}

'''
text = replace_once(text, insert_anchor, pending_helper + insert_anchor, "pending narrative helper")
path.write_text(text)
