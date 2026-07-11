from pathlib import Path
import re

p = Path('internal/services/security_radars.go')
s = p.read_text()
old = '''\tTokenSupply            float64
\tLargestHolderPct       int
\tTop10HolderPct         int
\tLargestAccounts        int
'''
new = '''\tTokenSupply            float64
\tLargestHolderPct       int
\tTop10HolderPct         int
\tRawLargestHolderPct    int
\tRawTop10HolderPct      int
\tLargestAccounts        int
\tHolderRoles            HolderRoleAnalysis
'''
if old not in s:
    raise SystemExit('radarEvidenceProfile holder fields not found')
s = s.replace(old, new, 1)
old = '''\tif largest, err := SolanaGetTokenLargestAccounts(ctx, rpcURL, req.Target); err == nil {
\t\tprofile.LiveRPC = true
\t\tprofile.IsTokenMint = true
\t\tprofile.LargestAccounts = len(largest.Value)
\t\tapplyLargestHolderEvidence(&profile, largest.Value)
\t} else {
\t\tprofile.Errors = append(profile.Errors, compactRadarError("getTokenLargestAccounts", err))
\t}
'''
new = '''\tif largest, err := SolanaGetTokenLargestAccounts(ctx, rpcURL, req.Target); err == nil {
\t\tprofile.LiveRPC = true
\t\tprofile.IsTokenMint = true
\t\tprofile.LargestAccounts = len(largest.Value)
\t\tapplyLargestHolderEvidence(&profile, largest.Value)
\t\tprofile.RawLargestHolderPct = profile.LargestHolderPct
\t\tprofile.RawTop10HolderPct = profile.Top10HolderPct
\t\tprofile.HolderRoles = AnalyzeSolanaHolderRoles(ctx, rpcURL, profile.TokenSupply, largest.Value)
\t\tif profile.HolderRoles.Available && profile.HolderRoles.RoleAdjusted && !profile.HolderRoles.BlockingEvidenceGap {
\t\t\tprofile.LargestHolderPct = int(math.Round(profile.HolderRoles.EffectiveTop1Percentage))
\t\t\tprofile.Top10HolderPct = int(math.Round(profile.HolderRoles.EffectiveTop10Percentage))
\t\t}
\t\tif profile.HolderRoles.BlockingEvidenceGap {
\t\t\tprofile.DataQuality = "partial_rpc_evidence"
\t\t\tprofile.EvidenceStatus = "dominant_holder_role_unresolved"
\t\t}
\t} else {
\t\tprofile.Errors = append(profile.Errors, compactRadarError("getTokenLargestAccounts", err))
\t}
'''
if old not in s:
    raise SystemExit('largest account collection block not found')
s = s.replace(old, new, 1)
old = '''\t\t"is_token_mint":         profile.IsTokenMint,
\t\t"latest_signature":      profile.LatestSignature,
'''
new = '''\t\t"is_token_mint":                  profile.IsTokenMint,
\t\t"largest_holder_percentage":       profile.LargestHolderPct,
\t\t"top_10_holder_percentage":         profile.Top10HolderPct,
\t\t"raw_largest_holder_percentage":   profile.RawLargestHolderPct,
\t\t"raw_top_10_holder_percentage":     profile.RawTop10HolderPct,
\t\t"holder_role_analysis":            profile.HolderRoles,
\t\t"holder_role_adjusted":            profile.HolderRoles.RoleAdjusted,
\t\t"holder_role_blocking_gap":        profile.HolderRoles.BlockingEvidenceGap,
\t\t"latest_signature":                profile.LatestSignature,
'''
if old not in s:
    raise SystemExit('base evidence insertion point not found')
p.write_text(s.replace(old, new, 1))

p = Path('internal/services/arvis_arms.go')
s = p.read_text()
old = '''func buildHolderArm(req SecurityRadarRequest, p radarEvidenceProfile, generatedAt string) SecurityRadarVerdict {
\tif !p.LiveRPC || !p.IsTokenMint || p.LargestAccounts == 0 {
\t\treturn unavailableArm("Holder Concentration", ModuleHolderConcentration, req, generatedAt, "Token largest-account evidence is required.")
\t}
\trisk := 5 + concentrationRisk(p.LargestHolderPct, p.Top10HolderPct)
\ts := armSignals(req, p, ModuleHolderConcentration)
\ts["largest_holder_percentage"] = p.LargestHolderPct
\ts["top_10_holder_percentage"] = p.Top10HolderPct
\ts["largest_accounts"] = p.LargestAccounts
\ts["token_supply"] = p.TokenSupply
\te := []string{fmt.Sprintf("Largest holder controls %d%% of observed supply.", p.LargestHolderPct), fmt.Sprintf("Top 10 holders control %d%%.", p.Top10HolderPct), fmt.Sprintf("Largest token accounts observed: %d.", p.LargestAccounts)}
\treturn evidenceArm("Holder Concentration", ModuleHolderConcentration, req, risk, s, e, generatedAt)
}
'''
new = '''func buildHolderArm(req SecurityRadarRequest, p radarEvidenceProfile, generatedAt string) SecurityRadarVerdict {
\tif !p.LiveRPC || !p.IsTokenMint || p.LargestAccounts == 0 {
\t\treturn unavailableArm("Holder Concentration", ModuleHolderConcentration, req, generatedAt, "Token largest-account evidence is required.")
\t}
\tif p.HolderRoles.BlockingEvidenceGap {
\t\tv := unavailableArm("Holder Concentration", ModuleHolderConcentration, req, generatedAt, "Dominant token-account role is unresolved; raw concentration cannot be converted into a wallet-control verdict.")
\t\tv.Signals["blocking_final_verdict"] = true
\t\tv.Signals["holder_role_analysis"] = p.HolderRoles
\t\tv.Signals["raw_largest_holder_percentage"] = p.RawLargestHolderPct
\t\tv.Signals["raw_top_10_holder_percentage"] = p.RawTop10HolderPct
\t\treturn v
\t}
\trisk := 5 + concentrationRisk(p.LargestHolderPct, p.Top10HolderPct)
\ts := armSignals(req, p, ModuleHolderConcentration)
\ts["largest_holder_percentage"] = p.LargestHolderPct
\ts["top_10_holder_percentage"] = p.Top10HolderPct
\ts["raw_largest_holder_percentage"] = p.RawLargestHolderPct
\ts["raw_top_10_holder_percentage"] = p.RawTop10HolderPct
\ts["holder_role_adjusted"] = p.HolderRoles.RoleAdjusted
\ts["holder_role_analysis"] = p.HolderRoles
\ts["largest_accounts"] = p.LargestAccounts
\ts["token_supply"] = p.TokenSupply
\tbasis := "raw total supply"
\tif p.HolderRoles.RoleAdjusted {
\t\tbasis = "role-adjusted circulating holder supply"
\t}
\te := []string{
\t\tfmt.Sprintf("Holder concentration basis: %s.", basis),
\t\tfmt.Sprintf("Risk-bearing largest holder controls %d%%; Top 10 control %d%%.", p.LargestHolderPct, p.Top10HolderPct),
\t\tfmt.Sprintf("Raw token-account concentration before role classification: Top 1=%d%% Top 10=%d%%.", p.RawLargestHolderPct, p.RawTop10HolderPct),
\t\tfmt.Sprintf("Protocol-controlled inventory=%.4f%%; burn sinks=%.4f%%; unresolved=%.4f%%.", p.HolderRoles.ProtocolControlledPercentage, p.HolderRoles.BurnPercentage, p.HolderRoles.UnresolvedPercentage),
\t}
\treturn evidenceArm("Holder Concentration", ModuleHolderConcentration, req, risk, s, e, generatedAt)
}
'''
if old not in s:
    raise SystemExit('buildHolderArm not found')
s = s.replace(old, new, 1)
old = '''func buildFinalArm(req SecurityRadarRequest, arms []SecurityRadarVerdict, generatedAt string) SecurityRadarVerdict {
\tverified := make([]SecurityRadarVerdict, 0, len(arms))
'''
new = '''func buildFinalArm(req SecurityRadarRequest, arms []SecurityRadarVerdict, generatedAt string) SecurityRadarVerdict {
\tfor _, arm := range arms {
\t\tif blocked, _ := arm.Signals["blocking_final_verdict"].(bool); blocked {
\t\t\tv := unavailableArm("Final Verdict Engine", ModuleFinalVerdictEngine, req, generatedAt, "A dominant holder role is unresolved; Unavailable is not Low and no final token-risk score is issued.")
\t\t\tv.Signals["blocking_final_verdict"] = true
\t\t\tv.Signals["blocking_module"] = arm.ModuleID
\t\t\treturn v
\t\t}
\t}
\tverified := make([]SecurityRadarVerdict, 0, len(arms))
'''
if old not in s:
    raise SystemExit('buildFinalArm insertion point not found')
p.write_text(s.replace(old, new, 1))

p = Path('internal/handlers/security_radar_detail.go')
s = p.read_text()
old = '''\tif network == "" {
\t\tnetwork = "solana-mainnet"
\t}

\tanalysis := services.AnalyzeArvisRadars'''
new = '''\tif network == "" {
\t\tnetwork = "solana-mainnet"
\t}
\tclassification := classifyRadarTarget(r.Context(), target)
\tif !radarTargetTokenVerdictAllowed(classification) {
\t\twriteJSON(w, http.StatusUnprocessableEntity, map[string]any{
\t\t\t"ok": false, "error": "target_not_token_mint", "message": radarTargetRejectionMessage(classification),
\t\t\t"target": target, "target_classification": classification,
\t\t\t"final_verdict": map[string]any{"risk_index": nil, "risk_level": "unknown", "grade": "-", "signed": false, "verdict": "INSUFFICIENT EVIDENCE"},
\t\t})
\t\treturn
\t}

\tanalysis := services.AnalyzeArvisRadars'''
if old not in s:
    raise SystemExit('detail target gate insertion point not found')
s = s.replace(old, new, 1)
pattern = re.compile(r'func radarDetailHolderDistribution\(parent context\.Context, target string\) map\[string\]any \{.*?\n\}\n\nfunc radarDetailTokenAmount', re.S)
replacement = '''func radarDetailHolderDistribution(parent context.Context, target string) map[string]any {
\trpcURL := strings.TrimSpace(firstNonEmptyString(os.Getenv("SOLANA_RPC_URL"), os.Getenv("ALCHEMY_SOLANA_RPC_URL")))
\tif rpcURL == "" {
\t\treturn map[string]any{"available": false, "status": "rpc_not_configured", "top_accounts": []any{}}
\t}
\tctx, cancel := context.WithTimeout(parent, 9*time.Second)
\tdefer cancel()
\tsupplyResult, err := services.SolanaGetTokenSupply(ctx, rpcURL, target)
\tif err != nil {
\t\treturn map[string]any{"available": false, "status": "supply_unavailable", "error": compactRadarDetailError(err), "top_accounts": []any{}}
\t}
\taccountsResult, err := services.SolanaGetTokenLargestAccounts(ctx, rpcURL, target)
\tif err != nil {
\t\treturn map[string]any{"available": false, "status": "largest_accounts_unavailable", "error": compactRadarDetailError(err), "top_accounts": []any{}}
\t}
\tsupply := radarDetailTokenAmount(supplyResult.Value)
\troles := services.AnalyzeSolanaHolderRoles(ctx, rpcURL, supply, accountsResult.Value)
\tout := services.HolderRoleAnalysisMap(roles)
\tout["decimals"] = supplyResult.Value.Decimals
\tout["largest_account_balance"] = 0.0
\tif len(accountsResult.Value) > 0 {
\t\tout["largest_account_balance"] = radarDetailRound(radarDetailTokenAmount(accountsResult.Value[0].SolanaTokenAmount), 6)
\t}
\tout["account_scope"] = "Token accounts resolved to owner wallets and owner programs; only positively identified protocol inventory or burn sinks are excluded from holder-risk concentration."
\treturn out
}

func radarDetailTokenAmount'''
s, count = pattern.subn(replacement, s, count=1)
if count != 1:
    raise SystemExit('radarDetailHolderDistribution replacement failed')
old = '''\treasons := []string{}
\tpositive := []string{}
\tlargest := radarDetailNumber(distribution["top_1_percentage"])
'''
new = '''\treasons := []string{}
\tpositive := []string{}
\troleAdjusted, _ := distribution["role_adjusted"].(bool)
\tblockingRoleGap, _ := distribution["blocking_evidence_gap"].(bool)
\tprotocolPct := radarDetailNumber(distribution["protocol_controlled_percentage"])
\tif blockingRoleGap {
\t\tlabel = "EVIDENCE_PENDING"
\t\treasons = append(reasons, "Baskın token hesabının ekonomik rolü çözülemedi; Koschei bu durumda yoğunlaşmayı LOW olarak yorumlamaz ve final karar bekletilir.")
\t}
\tif roleAdjusted && protocolPct > 0 {
\t\tpositive = append(positive, fmt.Sprintf("Ham arzın %.2f%%'si doğrulanmış protokol/bonding-curve envanteri olarak ayrıldı; holder riski dolaşımdaki cüzdan dağılımından hesaplandı.", protocolPct))
\t}
\tlargest := radarDetailNumber(distribution["top_1_percentage"])
'''
if old not in s:
    raise SystemExit('warning role insertion point not found')
p.write_text(s.replace(old, new, 1))

p = Path('internal/handlers/owner_operations.go')
s = p.read_text()
old = '''\tif available, _ := distribution["available"].(bool); available {
\t\tparts = append(parts, fmt.Sprintf("Holder yoğunluğu: Top 1 %v%%, Top 10 %v%%, Top 20 %v%%.", distribution["top_1_percentage"], distribution["top_10_percentage"], distribution["top_20_percentage"]))
\t}
'''
new = '''\tif available, _ := distribution["available"].(bool); available {
\t\tparts = append(parts, fmt.Sprintf("Holder yoğunluğu: Top 1 %v%%, Top 10 %v%%, Top 20 %v%%.", distribution["top_1_percentage"], distribution["top_10_percentage"], distribution["top_20_percentage"]))
\t\tif adjusted, _ := distribution["role_adjusted"].(bool); adjusted {
\t\t\tparts = append(parts, fmt.Sprintf("Ham arz yoğunluğu rol sınıflandırmasıyla düzeltildi: protokol/bonding-curve envanteri %v%%; baskın rol %v.", distribution["protocol_controlled_percentage"], distribution["dominant_role"]))
\t\t}
\t\tif blocked, _ := distribution["blocking_evidence_gap"].(bool); blocked {
\t\t\tparts = append(parts, "Baskın holder rolü çözülmediği için final yoğunlaşma kararı bekletildi; veri yokluğu düşük risk sayılmadı.")
\t\t}
\t}
'''
if old not in s:
    raise SystemExit('owner narrative holder block not found')
p.write_text(s.replace(old, new, 1))

p = Path('public/js/owner-control-center.js')
s = p.read_text()
old = "c.fillText('HOLDER CONCENTRATION',100,410);const vals="
new = "c.fillText(`HOLDER CONCENTRATION · ${dist.role_adjusted?'CIRCULATING':'RAW SUPPLY'}`,100,410);const vals="
if old not in s:
    raise SystemExit('poster holder heading not found')
s = s.replace(old, new, 1)
old = "c.strokeStyle='#1de6c833';c.strokeRect(70,600,1060,260);"
new = "if(dist.role_adjusted){c.fillStyle='#8fa4b5';c.font='16px Arial';c.fillText(`Protocol inventory excluded: ${Number(dist.protocol_controlled_percentage||0).toFixed(2)}% · ${String(dist.dominant_role||'protocol')}`,100,552)}c.strokeStyle='#1de6c833';c.strokeRect(70,600,1060,260);"
if old not in s:
    raise SystemExit('poster role note insertion point not found')
p.write_text(s.replace(old, new, 1))
