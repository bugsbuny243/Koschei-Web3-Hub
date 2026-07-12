from pathlib import Path
import re


def replace_once(text: str, old: str, new: str, label: str) -> str:
    if old not in text:
        raise SystemExit(f"missing replacement target: {label}")
    return text.replace(old, new, 1)

# Fix companion-file imports before gofmt/build.
flow_path = Path("internal/services/holder_cluster_flow_intelligence.go")
flow = flow_path.read_text()
flow = replace_once(flow, '\t"sort"\n\t"strings"', '\t"sort"\n\t"strconv"\n\t"strings"', "flow strconv import")
flow_path.write_text(flow)

# Wire flow observations into the bounded holder-cluster engine.
path = Path("internal/services/holder_cluster_intelligence.go")
text = path.read_text()
text = replace_once(
    text,
    'holderClusterWalletLimit    = 8\n\tholderClusterSignatureLimit = 20',
    'holderClusterWalletLimit             = 8\n\tholderClusterSignatureLimit          = 20\n\tholderClusterParsedTransactionLimit  = 3',
    "cluster constants",
)
text = replace_once(
    text,
    '\tFreshNearLaunch       bool     `json:"fresh_near_launch"`\n\tEvidence              []string `json:"evidence"`',
    '\tFreshNearLaunch       bool                           `json:"fresh_near_launch"`\n\tFlowObservations       []HolderClusterFlowObservation   `json:"flow_observations"`\n\tEvidence               []string                         `json:"evidence"`',
    "wallet flow field",
)
text = replace_once(
    text,
    '\tSynchronizedWallets       []string              `json:"synchronized_wallets"`\n\tFindings                  []string              `json:"findings"`',
    '\tSynchronizedWallets       []string                    `json:"synchronized_wallets"`\n\tFlow                      HolderClusterFlowAnalysis   `json:"flow"`\n\tFindings                  []string                    `json:"findings"`',
    "analysis flow field",
)
text = replace_once(
    text,
    '\tout.WalletsRequested = len(candidates)\n\tif len(candidates) < 3 {',
    '\tout.WalletsRequested = len(candidates)\n\tcandidateWallets := map[string]bool{}\n\tfor _, candidate := range candidates {\n\t\tcandidateWallets[candidate.OwnerWallet] = true\n\t}\n\tif len(candidates) < 3 {',
    "candidate wallet set",
)
text = replace_once(
    text,
    'analyzeHolderClusterWallet(ctx, rpcURL, mint, account, launchBlockTime)',
    'analyzeHolderClusterWallet(ctx, rpcURL, mint, account, launchBlockTime, candidateWallets)',
    "cluster wallet call",
)
text = replace_once(
    text,
    'func analyzeHolderClusterWallet(ctx context.Context, rpcURL, mint string, account HolderRoleAccount, launchBlockTime int64) HolderClusterWallet {',
    'func analyzeHolderClusterWallet(ctx context.Context, rpcURL, mint string, account HolderRoleAccount, launchBlockTime int64, holderWallets map[string]bool) HolderClusterWallet {',
    "cluster wallet signature",
)
text = replace_once(
    text,
    '\t\tStatus: "signature_history_unavailable", Evidence: []string{},\n\t}',
    '\t\tStatus: "signature_history_unavailable", FlowObservations: []HolderClusterFlowObservation{}, Evidence: []string{},\n\t}',
    "wallet flow initialization",
)
text = replace_once(
    text,
    '\t\tblockTime := holderClusterInt64(txMap["blockTime"])\n\t\tslot := holderClusterInt64(txMap["slot"])',
    '\t\tblockTime := holderClusterInt64(txMap["blockTime"])\n\t\tslot := holderClusterInt64(txMap["slot"])\n\t\trow.FlowObservations = append(row.FlowObservations, observeHolderClusterWalletFlow(txMap, signatures[index].Signature, mint, account.OwnerWallet, holderWallets)... )',
    "wallet flow observation",
)
text = replace_once(
    text,
    'func summarizeHolderCluster(out HolderClusterAnalysis) HolderClusterAnalysis {\n\tfunding := map[string][]HolderClusterWallet{}',
    'func summarizeHolderCluster(out HolderClusterAnalysis) HolderClusterAnalysis {\n\tout.Flow = summarizeHolderClusterFlow(out.Wallets)\n\tfunding := map[string][]HolderClusterWallet{}',
    "flow summary initialization",
)
text = replace_once(
    text,
    '\tfor _, wallet := range out.SynchronizedWallets {\n\t\tsuspicious[wallet] = true\n\t}',
    '\tfor _, wallet := range out.SynchronizedWallets {\n\t\tsuspicious[wallet] = true\n\t}\n\tfor _, wallet := range out.Flow.LinkedWallets {\n\t\tsuspicious[wallet] = true\n\t}',
    "flow linked supply union",
)
text = replace_once(
    text,
    '\tif out.LargestSharedFundingGroup >= 3 && out.SynchronizedWalletCount >= 3 {\n\t\tscore += 15\n\t}\n\tif score > 100 {',
    '\tif out.LargestSharedFundingGroup >= 3 && out.SynchronizedWalletCount >= 3 {\n\t\tscore += 15\n\t}\n\tscore += out.Flow.RiskContribution\n\tif out.Flow.LargestCommonExitGroup >= 3 && out.LargestSharedFundingGroup >= 2 {\n\t\tscore += 12\n\t}\n\tif score > 100 {',
    "flow score contribution",
)
text = replace_once(
    text,
    '\tcase out.LargestSharedFundingGroup >= 3 && out.SynchronizedWalletCount >= 3:\n\t\tout.Confidence = "high"\n\tcase out.LargestSharedFundingGroup >= 2 || out.SynchronizedWalletCount >= 3 || out.FreshWalletCount >= 3:',
    '\tcase out.LargestSharedFundingGroup >= 3 && out.SynchronizedWalletCount >= 3:\n\t\tout.Confidence = "high"\n\tcase out.Flow.Confidence == "high" && (out.LargestSharedFundingGroup >= 2 || out.SynchronizedWalletCount >= 2):\n\t\tout.Confidence = "high"\n\tcase out.LargestSharedFundingGroup >= 2 || out.SynchronizedWalletCount >= 3 || out.FreshWalletCount >= 3 || out.Flow.Confidence == "medium" || out.Flow.Confidence == "high":',
    "flow confidence",
)
text = replace_once(
    text,
    '\tout.Findings = holderClusterFindings(out)\n\tout.Limitations = append(out.Limitations,\n\t\t"Wallet history is bounded to the latest 20 signatures per holder and at most two parsed transactions per wallet.",',
    '\tout.Findings = holderClusterFindings(out)\n\tout.Limitations = append(out.Limitations, out.Flow.Limitations...)\n\tout.Limitations = append(out.Limitations,\n\t\tfmt.Sprintf("Wallet history is bounded to the latest %d signatures per holder and at most %d parsed transactions per wallet.", holderClusterSignatureLimit, holderClusterParsedTransactionLimit),',
    "flow limitations",
)
text = replace_once(
    text,
    '\tif len(findings) == 1 {\n\t\tfindings = append(findings, "No repeated shared-funding or synchronized-acquisition pattern was verified in the bounded observation window.")\n\t}\n\treturn findings',
    '\tfindings = append(findings, out.Flow.Findings...)\n\tif len(findings) == 1 {\n\t\tfindings = append(findings, "No repeated shared-funding, synchronized-acquisition, common-exit or internal-transfer pattern was verified in the bounded observation window.")\n\t}\n\treturn findings',
    "flow findings",
)

transaction_pattern = re.compile(r'func holderClusterTransactionIndexes\(signatures \[\]SolanaSignatureInfo, launchBlockTime int64\) \[\]int \{.*?\n\}', re.S)
transaction_replacement = '''func holderClusterTransactionIndexes(signatures []SolanaSignatureInfo, launchBlockTime int64) []int {
	if len(signatures) == 0 {
		return nil
	}
	indexes := []int{}
	seen := map[int]bool{}
	appendIndex := func(index int) {
		if index < 0 || index >= len(signatures) || seen[index] || signatures[index].Err != nil || strings.TrimSpace(signatures[index].Signature) == "" {
			return
		}
		seen[index] = true
		indexes = append(indexes, index)
	}

	// Newest successful transaction captures recent exit/transfer behavior.
	for i := 0; i < len(signatures); i++ {
		if signatures[i].Err == nil && strings.TrimSpace(signatures[i].Signature) != "" {
			appendIndex(i)
			break
		}
	}
	// Oldest bounded transaction captures initial funding/age evidence.
	for i := len(signatures) - 1; i >= 0; i-- {
		if signatures[i].Err == nil && strings.TrimSpace(signatures[i].Signature) != "" {
			appendIndex(i)
			break
		}
	}
	// Closest transaction to the bounded launch estimate captures acquisition timing.
	if launchBlockTime > 0 {
		closest, best := -1, int64(math.MaxInt64)
		for i, signature := range signatures {
			if signature.Err != nil || signature.BlockTime == nil || *signature.BlockTime <= 0 || strings.TrimSpace(signature.Signature) == "" {
				continue
			}
			delta := *signature.BlockTime - launchBlockTime
			if delta < 0 {
				delta = -delta
			}
			if delta < best {
				best, closest = delta, i
			}
		}
		appendIndex(closest)
	}
	if len(indexes) > holderClusterParsedTransactionLimit {
		indexes = indexes[:holderClusterParsedTransactionLimit]
	}
	return indexes
}'''
text, count = transaction_pattern.subn(transaction_replacement, text, count=1)
if count != 1:
    raise SystemExit("holderClusterTransactionIndexes replacement failed")
path.write_text(text)

# Expose flow metrics in the signed Funding Cluster arm.
path = Path("internal/services/arvis_arms.go")
text = path.read_text()
text = replace_once(
    text,
    '\ts["linked_holder_percentage"] = a.LinkedHolderPercentage\n\ts["sybil_verdict"] = a.Verdict',
    '\ts["linked_holder_percentage"] = a.LinkedHolderPercentage\n\ts["common_exit_group_count"] = a.Flow.CommonExitGroupCount\n\ts["largest_common_exit_group"] = a.Flow.LargestCommonExitGroup\n\ts["internal_transfer_count"] = a.Flow.InternalTransferCount\n\ts["circular_wallet_count"] = a.Flow.CircularWalletCount\n\ts["flow_linked_holder_percentage"] = a.Flow.LinkedHolderPercentage\n\ts["holder_flow_confidence"] = a.Flow.Confidence\n\ts["sybil_verdict"] = a.Verdict',
    "funding arm flow metrics",
)
text = replace_once(
    text,
    'v.Recommendation = "Inspect shared funders, synchronized acquisition slots and linked holder wallets before relying on apparent decentralization."',
    'v.Recommendation = "Inspect shared funders, synchronized acquisition slots, common exits and direct holder transfers before relying on apparent decentralization."',
    "funding arm recommendation",
)
path.write_text(text)

# Expand the visible Owner Holder Cluster Intelligence card.
path = Path("public/js/owner-control-center.js")
text = path.read_text()
pattern = re.compile(r'function holderClusterHTML\(raw\)\{.*?\}\nfunction radarReportHTML', re.S)
replacement = r'''function holderClusterHTML(raw){const c=obj(raw);if(!Object.keys(c).length)return'';const flow=obj(c.flow),findings=arr(c.findings),limits=arr(c.limitations),flowFindings=arr(flow.findings),flowLimits=arr(flow.limitations),commonExits=arr(flow.common_exit_groups),internal=arr(flow.internal_transfers),status=c.available?(c.risk_level||c.status||'unknown'):'insufficient_evidence',flowStatus=flow.available?(flow.status||'unknown'):'insufficient_evidence';return`<details class="owner-details section-gap" open><summary><span><b>Holder Cluster Intelligence V2</b><small>Owner wallet → funding → acquisition slot → ortak çıkış → iç transfer → bağlı arz.</small></span><span>⌄</span></summary><div class="grid compact-grid section-gap">${kpi('İstenen cüzdan',num(c.wallets_requested),'Risk taşıyan owner wallet','tone-cyan','◎')}${kpi('Parsed kanıt',num(c.wallets_analyzed),'Gerçek transaction incelenen','tone-cyan','◇')}${kpi('Ortak fonlayıcı',num(c.shared_funding_group_count),'Tek başına ortak sahiplik kanıtı değildir',c.shared_funding_group_count?'tone-amber':'tone-green','◈')}${kpi('Senkron alım',num(c.synchronized_wallet_count),'Aynı slot penceresinde',c.synchronized_wallet_count?'tone-amber':'tone-green','◉')}${kpi('Bağlı arz',`${num(c.linked_holder_percentage)}%`,'Funding + timing + flow ilişkileri',Number(c.linked_holder_percentage)>=20?'tone-red':'tone-cyan','%')}${kpi('Confidence',String(c.confidence||'none').toUpperCase(),c.verdict||c.status||'Kanıt kapsamı','tone-cyan','◆')}</div><div class="card-head section-gap"><div><span class="eyebrow">Cluster durumu</span><h3>${esc(c.verdict||label(status))}</h3></div>${badge(status)}</div>${findings.length?`<div class="clean-list">${findings.map((x,i)=>`<div class="summary-row"><span>#${i+1}</span><b style="text-align:left">${esc(x)}</b>${badge('verified')}</div>`).join('')}</div>`:''}<div class="card-head section-gap"><div><span class="eyebrow">Holder Flow Intelligence</span><h3>Ortak çıkış ve cüzdanlar arası hareket</h3></div>${badge(flowStatus)}</div><div class="grid compact-grid">${kpi('Outflow işlem',num(flow.transactions_with_outflow),'Hedef token çıkışı görülen transaction','tone-cyan','↗')}${kpi('Ortak çıkış',num(flow.common_exit_group_count),'Aynı recipient owner','tone-amber','⇢')}${kpi('İç transfer',num(flow.internal_transfer_count),'Top holder ownerları arasında','tone-amber','⇄')}${kpi('Döngüsel cüzdan',num(flow.circular_wallet_count),'Wash trading hükmü değildir',flow.circular_wallet_count?'tone-amber':'tone-green','↻')}${kpi('Flow bağlı arz',`${num(flow.linked_holder_percentage)}%`,'Flow ilişkisine giren holder payı',Number(flow.linked_holder_percentage)>=20?'tone-red':'tone-cyan','%')}${kpi('Flow confidence',String(flow.confidence||'none').toUpperCase(),`Risk katkısı ${num(flow.risk_contribution)}/45`,'tone-cyan','◆')}</div>${flowFindings.length?`<div class="clean-list section-gap">${flowFindings.map((x,i)=>`<div class="summary-row"><span>F${i+1}</span><b style="text-align:left">${esc(x)}</b>${badge('verified')}</div>`).join('')}</div>`:''}${commonExits.length?`<div class="table-wrap section-gap"><table class="table"><thead><tr><th>Ortak çıkış recipient</th><th>Cüzdan</th><th>Bağlı arz</th></tr></thead><tbody>${commonExits.map(g=>`<tr><td class="mono">${esc(short(g.key,34))}</td><td>${num(g.member_count)}</td><td>${num(g.holder_percentage)}%</td></tr>`).join('')}</tbody></table></div>`:''}${internal.length?`<div class="table-wrap section-gap"><table class="table"><thead><tr><th>Kaynak holder</th><th>Hedef holder</th><th>Miktar</th><th>Slot</th></tr></thead><tbody>${internal.slice(0,20).map(x=>`<tr><td class="mono">${esc(short(x.source_wallet,28))}</td><td class="mono">${esc(short(x.destination,28))}</td><td>${num(x.amount)}</td><td>${num(x.slot)}</td></tr>`).join('')}</tbody></table></div>`:''}${limits.length?`<div class="warning-box section-gap"><b>Cluster sınırları</b><br>${limits.map(esc).join(' · ')}</div>`:''}${flowLimits.length?`<div class="warning-box section-gap"><b>Flow sınırları</b><br>${flowLimits.map(esc).join(' · ')}</div>`:''}</details>`}
function radarReportHTML'''
text, count = pattern.subn(replacement, text, count=1)
if count != 1:
    raise SystemExit("holderClusterHTML replacement failed")
path.write_text(text)

# Force fresh Owner JS after deploy.
path = Path("public/owner-production.html")
text = path.read_text()
text = replace_once(text, '/js/owner-control-center.js?v=6', '/js/owner-control-center.js?v=7', "owner cache version")
path.write_text(text)
