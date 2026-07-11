from pathlib import Path


def replace_once(text: str, old: str, new: str, label: str) -> str:
    if old not in text:
        raise SystemExit(f"missing replacement target: {label}")
    return text.replace(old, new, 1)

# 1) Track whether the mint signature history is complete enough to support timing claims.
path = Path("internal/services/security_radars.go")
text = path.read_text()
text = replace_once(
    text,
    "\tSignatureWindowSeconds int64\n\tLatestSignature        string",
    "\tSignatureWindowSeconds          int64\n\tTargetSignatureHistoryExhausted bool\n\tTargetSignatureTimingObserved   bool\n\tLatestSignature                 string",
    "signature evidence fields",
)
text = replace_once(
    text,
    "\t\tprofile.RecentSignatureCount = len(signatures)\n\t\tif len(signatures) > 0 {",
    "\t\tprofile.RecentSignatureCount = len(signatures)\n\t\tprofile.TargetSignatureHistoryExhausted = len(signatures) < 100\n\t\tif len(signatures) > 0 {",
    "signature history exhaustion",
)
text = replace_once(
    text,
    "\t\tif newest > 0 && oldest > 0 && newest >= oldest {\n\t\t\tprofile.SignatureWindowSeconds = newest - oldest",
    "\t\tif newest > 0 && oldest > 0 && newest >= oldest {\n\t\t\tprofile.TargetSignatureTimingObserved = true\n\t\t\tprofile.SignatureWindowSeconds = newest - oldest",
    "signature timing observation",
)
path.write_text(text)

# 2) Make majority-wallet concentration high risk and reject truncated mint windows as launch timing.
path = Path("internal/services/arvis_arms.go")
text = path.read_text()
text = replace_once(
    text,
    "\trisk := 5 + concentrationRisk(p.LargestHolderPct, p.Top10HolderPct)\n\ts := armSignals(req, p, ModuleHolderConcentration)",
    "\trisk := 5 + concentrationRisk(p.LargestHolderPct, p.Top10HolderPct)\n\tswitch {\n\tcase p.LargestHolderPct >= 50:\n\t\tif risk < 70 {\n\t\t\trisk = 70\n\t\t}\n\tcase p.LargestHolderPct >= 35:\n\t\tif risk < 55 {\n\t\t\trisk = 55\n\t\t}\n\tcase p.Top10HolderPct >= 75:\n\t\tif risk < 60 {\n\t\t\trisk = 60\n\t\t}\n\t}\n\ts := armSignals(req, p, ModuleHolderConcentration)",
    "holder concentration severity floor",
)
old_sniper = '''func buildSniperTimingArm(req SecurityRadarRequest, p radarEvidenceProfile, generatedAt string) SecurityRadarVerdict {
	mintTiming := p.LiveRPC && p.RecentSignatureCount > 0 && p.SignatureWindowSeconds > 0
	clusterTiming := p.HolderCluster.Available && p.HolderCluster.SynchronizedWalletCount >= 2
	if !mintTiming && !clusterTiming {
		return unavailableArm("Sniper Timing Detector", ModuleSniperTimingDetector, req, generatedAt, "Timestamped mint activity or parsed holder acquisition slots are required.")
	}
	risk := 8
	s := armSignals(req, p, ModuleSniperTimingDetector)
	e := []string{}
	if mintTiming {
		risk += burstRisk(p.RecentSignatureCount, p.SignatureWindowSeconds)
		s["recent_signature_count"] = p.RecentSignatureCount
		s["signature_window_seconds"] = p.SignatureWindowSeconds
		s["failed_signature_count"] = p.FailedSignatureCount
		e = append(e, fmt.Sprintf("Observed %d mint-address signatures in a %d second window.", p.RecentSignatureCount, p.SignatureWindowSeconds))
	}
	if clusterTiming {
		clusterRisk := 12 + p.HolderCluster.SynchronizedWalletCount*10
		if p.HolderCluster.SynchronizationSlotSpread <= 1 {
			clusterRisk += 15
		}
		if clusterRisk > risk {
			risk = clusterRisk
		}
		s["synchronized_holder_wallets"] = p.HolderCluster.SynchronizedWallets
		s["synchronized_wallet_count"] = p.HolderCluster.SynchronizedWalletCount
		s["synchronization_slot_spread"] = p.HolderCluster.SynchronizationSlotSpread
		s["parsed_holder_acquisition_evidence"] = true
		e = append(e, fmt.Sprintf("%d resolved holder wallets acquired the token inside a %d-slot window.", p.HolderCluster.SynchronizedWalletCount, p.HolderCluster.SynchronizationSlotSpread))
	}
	s["scope_note"] = "Timing coordination is evidence of automation/coordination risk; it is not sole proof of common ownership."
	e = append(e, "Timing evidence is combined with funding relations before Koschei raises a high-confidence Sybil conclusion.")
	return evidenceArm("Sniper Timing Detector", ModuleSniperTimingDetector, req, risk, s, e, generatedAt)
}'''
new_sniper = '''func buildSniperTimingArm(req SecurityRadarRequest, p radarEvidenceProfile, generatedAt string) SecurityRadarVerdict {
	mintTiming := p.LiveRPC && p.TargetSignatureHistoryExhausted && p.TargetSignatureTimingObserved && p.RecentSignatureCount >= 2
	clusterTiming := p.HolderCluster.Available && p.HolderCluster.SynchronizedWalletCount >= 2
	if !mintTiming && !clusterTiming {
		reason := "Parsed holder acquisition slots or a complete mint-address signature history are required."
		if p.RecentSignatureCount >= 100 && !p.TargetSignatureHistoryExhausted {
			reason = "The latest 100 mint-address signatures are a truncated recent-activity window, not launch timing; parsed holder acquisition slots are required."
		}
		return unavailableArm("Sniper Timing Detector", ModuleSniperTimingDetector, req, generatedAt, reason)
	}
	risk := 8
	s := armSignals(req, p, ModuleSniperTimingDetector)
	e := []string{}
	if mintTiming {
		risk += burstRisk(p.RecentSignatureCount, p.SignatureWindowSeconds)
		s["recent_signature_count"] = p.RecentSignatureCount
		s["signature_window_seconds"] = p.SignatureWindowSeconds
		s["failed_signature_count"] = p.FailedSignatureCount
		s["mint_signature_history_exhausted"] = true
		s["mint_timing_scope"] = "complete_observed_address_history"
		e = append(e, fmt.Sprintf("RPC returned the complete observed mint-address history: %d signatures across %d seconds.", p.RecentSignatureCount, p.SignatureWindowSeconds))
	}
	if clusterTiming {
		clusterRisk := 12 + p.HolderCluster.SynchronizedWalletCount*10
		if p.HolderCluster.SynchronizationSlotSpread <= 1 {
			clusterRisk += 15
		}
		if clusterRisk > risk {
			risk = clusterRisk
		}
		s["synchronized_holder_wallets"] = p.HolderCluster.SynchronizedWallets
		s["synchronized_wallet_count"] = p.HolderCluster.SynchronizedWalletCount
		s["synchronization_slot_spread"] = p.HolderCluster.SynchronizationSlotSpread
		s["parsed_holder_acquisition_evidence"] = true
		e = append(e, fmt.Sprintf("%d resolved holder wallets acquired the token inside a %d-slot window.", p.HolderCluster.SynchronizedWalletCount, p.HolderCluster.SynchronizationSlotSpread))
	}
	if p.RecentSignatureCount >= 100 && !p.TargetSignatureHistoryExhausted {
		s["truncated_recent_mint_window_ignored"] = true
		e = append(e, "The latest 100 mint-address signatures were not treated as launch timing because the history was truncated.")
	}
	s["scope_note"] = "Timing coordination is evidence of automation/coordination risk; it is not sole proof of common ownership."
	e = append(e, "Timing evidence is combined with funding relations before Koschei raises a high-confidence Sybil conclusion.")
	return evidenceArm("Sniper Timing Detector", ModuleSniperTimingDetector, req, risk, s, e, generatedAt)
}'''
text = replace_once(text, old_sniper, new_sniper, "sniper timing evidence policy")
text = replace_once(
    text,
    '''\tif !a.Available {\n\t\treturn unavailableArm("Funding Cluster Detector", ModuleFundingClusterDetector, req, generatedAt, "At least three resolved holder wallets with bounded funding evidence are required; unavailable evidence is not LOW.")\n\t}''',
    '''\tif !a.Available {\n\t\tv := unavailableArm("Funding Cluster Detector", ModuleFundingClusterDetector, req, generatedAt, "At least three resolved holder wallets with parsed funding evidence are required; unavailable evidence is not LOW.")\n\t\tv.Signals["holder_cluster_analysis"] = a\n\t\tv.Signals["cluster_confidence"] = a.Confidence\n\t\tfor _, limitation := range a.Limitations {\n\t\t\tv.Evidence = append(v.Evidence, "Limitation: "+limitation)\n\t\t}\n\t\treturn v\n\t}''',
    "unavailable holder cluster detail",
)
path.write_text(text)

# 3) Preserve richer holder-wallet cluster evidence during transaction enrichment.
path = Path("internal/services/arvis_transaction_bundle.go")
text = path.read_text()
text = replace_once(
    text,
    "\treplaceArvisArm(arms, buildFundingClusterTransactionArm(req, txEvidence, generatedAt))",
    "\treplaceFundingClusterArmPreservingHolderEvidence(arms, buildFundingClusterTransactionArm(req, txEvidence, generatedAt))",
    "transaction bundle funding precedence",
)
helper = '''
func replaceFundingClusterArmPreservingHolderEvidence(arms []SecurityRadarVerdict, replacement SecurityRadarVerdict) {
	for i := range arms {
		if arms[i].ModuleID != ModuleFundingClusterDetector {
			continue
		}
		_, hasHolderCluster := arms[i].Signals["holder_cluster_analysis"]
		if hasHolderCluster {
			if arms[i].Signed && SecurityRadarVerdictHasVerifiedEvidence(arms[i]) {
				return
			}
			if !replacement.Signed {
				return
			}
		}
		if !replacement.Signed && arms[i].Signed && SecurityRadarVerdictHasVerifiedEvidence(arms[i]) {
			return
		}
		arms[i] = replacement
		return
	}
}

'''
text = replace_once(text, "func replaceArvisArmPreservingVerifiedSource", helper + "func replaceArvisArmPreservingVerifiedSource", "funding precedence helper")
path.write_text(text)

path = Path("internal/services/arvis_transaction_arms.go")
text = path.read_text()
text = replace_once(
    text,
    "\treplaceArvisArm(arms, buildFundingClusterTransactionArm(req, txEvidence, generatedAt))",
    "\treplaceFundingClusterArmPreservingHolderEvidence(arms, buildFundingClusterTransactionArm(req, txEvidence, generatedAt))",
    "direct transaction analysis funding precedence",
)
path.write_text(text)

# 4) Make owner narrative and health status reflect actual evidence and RPC saver mode.
path = Path("internal/handlers/owner_operations.go")
text = path.read_text()
text = replace_once(text, '"net/http"\n\t"strings"', '"net/http"\n\t"os"\n\t"strings"', "owner operations os import")
text = replace_once(
    text,
    "\tradar := h.securityRadarStreamStats(ctx)\n\tservicesMap := map[string]any{",
    "\tradar := h.securityRadarStreamStats(ctx)\n\tradarStatus := firstMapString(radar, \"pipeline_status\")\n\tif strings.EqualFold(strings.TrimSpace(os.Getenv(\"SOLANA_RPC_LIMIT_SAVER_ENABLED\")), \"true\") {\n\t\tradarStatus = \"manual_rpc_saver\"\n\t\tradar[\"pipeline_status\"] = radarStatus\n\t\tradar[\"background_streams_paused\"] = true\n\t\tradar[\"manual_scans_available\"] = true\n\t}\n\tservicesMap := map[string]any{",
    "owner saver-aware radar status",
)
text = replace_once(
    text,
    '"security_radar":  map[string]any{"status": firstMapString(radar, "pipeline_status")},',
    '"security_radar":  map[string]any{"status": radarStatus},',
    "owner service radar status",
)
text = replace_once(text, "parts = append(parts, ownerRadarPracticalConclusion(level))", "parts = append(parts, ownerRadarPracticalConclusion(level, distribution))", "contextual practical conclusion call")
old_practical = '''func ownerRadarPracticalConclusion(level string) string {
	switch level {
	case "critical", "high":
		return "Pratik sonuç: işlem yapmadan önce ana risk sürücüsünün kanıtlarını, likidite çıkış yollarını, creator geçmişini ve bağlı cüzdan kümelerini doğrulamak gerekir."
	case "medium":
		return "Pratik sonuç: holder dağılımı rahat görünse bile token otomatik olarak güvenli sayılmaz. Ana risk modülü, likidite davranışı, creator geçmişi ve Sybil/funding-cluster bağlantıları birlikte incelenmelidir."
	case "low":
		return "Pratik sonuç: mevcut taramada ağır bir alarm yok; yine de likidite, creator ve cüzdan kümeleri değişebileceği için karar güncel verilerle yenilenmelidir."
	default:
		return "Pratik sonuç: kanıt kapsamı tamamlanmadan kesin güvenlik yorumu yapılmamalıdır."
	}
}'''
new_practical = '''func ownerRadarPracticalConclusion(level string, distribution map[string]any) string {
	available, _ := distribution["available"].(bool)
	if available {
		top1 := radarDetailNumber(distribution["top_1_percentage"])
		top10 := radarDetailNumber(distribution["top_10_percentage"])
		switch {
		case top1 >= 50:
			return "Pratik sonuç: tek bir risk taşıyan cüzdan dolaşımdaki arzın yarısından fazlasını kontrol ediyor. Bu cüzdanın satış, transfer, funding ve ortak-exit geçmişi çözülmeden token güvenli kabul edilmemelidir."
		case top1 >= 20 || top10 >= 75:
			return "Pratik sonuç: holder dağılımı merkezileşmiş durumda. Büyük cüzdanların satış kapasitesi, likidite çıkış yolları ve cluster bağlantıları işlem öncesinde doğrulanmalıdır."
		}
	}
	switch level {
	case "critical", "high":
		return "Pratik sonuç: işlem yapmadan önce ana risk sürücüsünün kanıtlarını, likidite çıkış yollarını, creator geçmişini ve bağlı cüzdan kümelerini doğrulamak gerekir."
	case "medium":
		return "Pratik sonuç: token otomatik olarak güvenli sayılmaz. Ana risk modülü, likidite davranışı, creator geçmişi ve Sybil/funding-cluster bağlantıları birlikte incelenmelidir."
	case "low":
		return "Pratik sonuç: mevcut taramada ağır bir alarm yok; yine de likidite, creator ve cüzdan kümeleri değişebileceği için karar güncel verilerle yenilenmelidir."
	default:
		return "Pratik sonuç: kanıt kapsamı tamamlanmadan kesin güvenlik yorumu yapılmamalıdır."
	}
}'''
text = replace_once(text, old_practical, new_practical, "contextual practical conclusion")
path.write_text(text)

# 5) Fix owner UI evidence labels and expose holder-cluster details.
path = Path("public/js/owner-control-center.js")
text = path.read_text()
text = replace_once(text, "unknown:'Bilinmiyor',missing:'Eksik'", "unknown:'Bilinmiyor','insufficient evidence':'Kanıt Yetersiz','manual rpc saver':'Manuel · RPC Tasarrufu',missing:'Eksik'", "owner status labels")
cluster_helper = '''function holderClusterHTML(raw){const c=obj(raw);if(!Object.keys(c).length)return'';const findings=arr(c.findings),limits=arr(c.limitations),status=c.available?(c.risk_level||c.status||'unknown'):'insufficient_evidence';return`<details class="owner-details section-gap" open><summary><span><b>Holder Cluster Intelligence</b><small>Owner wallet → funding → acquisition slot → bağlı arz.</small></span><span>⌄</span></summary><div class="grid compact-grid section-gap">${kpi('İstenen cüzdan',num(c.wallets_requested),'Risk taşıyan owner wallet','tone-cyan','◎')}${kpi('Parsed kanıt',num(c.wallets_analyzed),'Gerçek transaction incelenen','tone-cyan','◇')}${kpi('Ortak fonlayıcı',num(c.shared_funding_group_count),'Tek başına ortak sahiplik kanıtı değildir',c.shared_funding_group_count?'tone-amber':'tone-green','◈')}${kpi('Senkron alım',num(c.synchronized_wallet_count),'Aynı slot penceresinde',c.synchronized_wallet_count?'tone-amber':'tone-green','◉')}${kpi('Bağlı arz',`${num(c.linked_holder_percentage)}%`,'İlişki sinyali taşıyan holder payı',Number(c.linked_holder_percentage)>=20?'tone-red':'tone-cyan','%')}${kpi('Confidence',String(c.confidence||'none').toUpperCase(),c.verdict||c.status||'Kanıt kapsamı','tone-cyan','◆')}</div><div class="card-head section-gap"><div><span class="eyebrow">Cluster durumu</span><h3>${esc(c.verdict||label(status))}</h3></div>${badge(status)}</div>${findings.length?`<div class="clean-list">${findings.map((x,i)=>`<div class="summary-row"><span>#${i+1}</span><b style="text-align:left">${esc(x)}</b>${badge('verified')}</div>`).join('')}</div>`:''}${limits.length?`<div class="warning-box section-gap"><b>Sınırlar</b><br>${limits.map(esc).join(' · ')}</div>`:''}</details>`}
'''
text = replace_once(text, "function radarReportHTML", cluster_helper + "function radarReportHTML", "holder cluster UI helper")
text = replace_once(text, "struct=obj(d.structural_memory),mods=arr(d.modules),evidence=arr(d.evidence)", "struct=obj(d.structural_memory),mods=arr(d.modules),evidence=arr(d.evidence),cluster=obj(d.holder_cluster)", "holder cluster report binding")
text = replace_once(text, "</details><details class=\"owner-details section-gap\" open><summary><span><b>Complete evidence log", "</details>${holderClusterHTML(cluster)}<details class=\"owner-details section-gap\" open><summary><span><b>Complete evidence log", "holder cluster report section")
text = replace_once(text, "${badge('verified')}</div>`).join('')||'<div class=\"empty\">Doğrulanmış kanıt satırı yok.</div>'", "${badge(x&&typeof x==='object'&&x.verified===true?'verified':'unavailable')}</div>`).join('')||'<div class=\"empty\">Kanıt satırı yok.</div>'", "evidence verification label")
path.write_text(text)

# 6) Add focused tests.
Path("internal/services/arvis_evidence_precedence_test.go").write_text(r'''package services

import (
	"strings"
	"testing"
	"time"
)

func TestFundingClusterHolderEvidenceSurvivesTransactionEnrichment(t *testing.T) {
	req := SecurityRadarRequest{Target: "mint", Network: "solana-mainnet"}
	base := evidenceArm("Funding Cluster Detector", ModuleFundingClusterDetector, req, 72, map[string]any{
		"real_onchain_evidence": true,
		"holder_cluster_analysis": HolderClusterAnalysis{Available: true, WalletsAnalyzed: 5},
	}, []string{"holder cluster"}, time.Now().UTC().Format(time.RFC3339))
	replacement := evidenceArm("Funding Cluster Detector", ModuleFundingClusterDetector, req, 22, map[string]any{
		"real_onchain_evidence": true,
		"transaction_signature": "sig",
	}, []string{"initialization delta"}, time.Now().UTC().Format(time.RFC3339))
	arms := []SecurityRadarVerdict{base}
	replaceFundingClusterArmPreservingHolderEvidence(arms, replacement)
	if arms[0].RiskIndex != 72 {
		t.Fatalf("holder cluster was overwritten: %#v", arms[0])
	}
}

func TestFundingClusterTransactionFillsUnavailableBase(t *testing.T) {
	req := SecurityRadarRequest{Target: "mint", Network: "solana-mainnet"}
	base := unavailableArm("Funding Cluster Detector", ModuleFundingClusterDetector, req, time.Now().UTC().Format(time.RFC3339), "missing")
	replacement := evidenceArm("Funding Cluster Detector", ModuleFundingClusterDetector, req, 22, map[string]any{"real_onchain_evidence": true}, []string{"initialization delta"}, time.Now().UTC().Format(time.RFC3339))
	arms := []SecurityRadarVerdict{base}
	replaceFundingClusterArmPreservingHolderEvidence(arms, replacement)
	if !arms[0].Signed || arms[0].RiskIndex != 22 {
		t.Fatalf("verified transaction evidence did not fill unavailable base: %#v", arms[0])
	}
}

func TestSniperTimingRejectsTruncatedLatestHundred(t *testing.T) {
	arm := buildSniperTimingArm(SecurityRadarRequest{Target: "mint", Network: "solana-mainnet"}, radarEvidenceProfile{
		LiveRPC: true, RecentSignatureCount: 100, SignatureWindowSeconds: 1,
		TargetSignatureHistoryExhausted: false, TargetSignatureTimingObserved: true,
	}, time.Now().UTC().Format(time.RFC3339))
	if arm.Signed || arm.RiskLevel != "unknown" {
		t.Fatalf("truncated recent window must not become sniper timing: %#v", arm)
	}
	if !strings.Contains(strings.Join(arm.Evidence, " "), "truncated") {
		t.Fatalf("expected truncated-window explanation: %#v", arm.Evidence)
	}
}

func TestSniperTimingAcceptsCompleteMintHistory(t *testing.T) {
	arm := buildSniperTimingArm(SecurityRadarRequest{Target: "mint", Network: "solana-mainnet"}, radarEvidenceProfile{
		LiveRPC: true, RecentSignatureCount: 40, SignatureWindowSeconds: 8,
		TargetSignatureHistoryExhausted: true, TargetSignatureTimingObserved: true,
	}, time.Now().UTC().Format(time.RFC3339))
	if !arm.Signed {
		t.Fatalf("complete observed history should be usable: %#v", arm)
	}
}

func TestMajorityEOAHolderIsHighRisk(t *testing.T) {
	arm := buildHolderArm(SecurityRadarRequest{Target: "mint", Network: "solana-mainnet"}, radarEvidenceProfile{
		LiveRPC: true, IsTokenMint: true, LargestAccounts: 20,
		LargestHolderPct: 59, Top10HolderPct: 64,
	}, time.Now().UTC().Format(time.RFC3339))
	if !arm.Signed || arm.RiskIndex < 65 || arm.RiskLevel != "high" {
		t.Fatalf("majority holder must not be LOW: %#v", arm)
	}
}
''')

Path("internal/handlers/owner_radar_conclusion_test.go").write_text(r'''package handlers

import (
	"strings"
	"testing"
)

func TestOwnerRadarPracticalConclusionReflectsMajorityHolder(t *testing.T) {
	got := ownerRadarPracticalConclusion("medium", map[string]any{
		"available": true,
		"top_1_percentage": 58.69,
		"top_10_percentage": 63.66,
	})
	if strings.Contains(strings.ToLower(got), "rahat") || !strings.Contains(got, "yarısından fazlasını") {
		t.Fatalf("contradictory practical conclusion: %s", got)
	}
}
''')
