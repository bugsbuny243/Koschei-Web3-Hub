from pathlib import Path
import re


def replace_once(text: str, old: str, new: str, label: str) -> str:
    if old not in text:
        raise SystemExit(f"missing replacement target: {label}")
    return text.replace(old, new, 1)

# PumpPortal becomes discovery-only; the selective volume worker decides which
# mints deserve expensive full RPC enrichment.
path = Path("internal/services/pumpportal_radar_adapter.go")
text = path.read_text()
start = text.index("\t// A PumpPortal discovery must become a complete ARVIS story")
end_marker = "\treturn firstErr\n}"
end = text.index(end_marker, start) + len(end_marker)
text = text[:start] + '''\t// Discovery is intentionally cheap. PumpPortal new-token/migration events
\t// are stored for coverage, while the selective 24h USD volume worker decides
\t// whether this mint may consume Solana RPC for a complete ARVIS report.
\t// HARD RULE: do not run a full scan for every new Pump token.
\treturn nil
}''' + text[end:]
old_start = '''func StartPumpPortalRadarIfEnabled(ctx context.Context, db *sql.DB) func() {
\tcfg := LoadPumpPortalConfigFromEnv()
\tif !cfg.Enabled {
\t\treturn func() {}
\t}
\tif db == nil {
\t\tlog.Printf("pumpportal radar disabled: database unavailable")
\t\treturn func() {}
\t}
\tctx, cancel := context.WithCancel(ctx)
\tstore := NewSecurityRadarStore(db)
\tadapter := NewPumpPortalRadarAdapter(store)
\tclient := NewPumpPortalClient(cfg)
\tgo client.Start(ctx, adapter.HandleEvent)
\tlog.Printf("pumpportal radar discovery started: data-only websocket=%s", cfg.redactedWebsocketHost())
\treturn cancel
}'''
new_start = '''func StartPumpPortalRadarIfEnabled(ctx context.Context, db *sql.DB) func() {
\tcfg := LoadPumpPortalConfigFromEnv()
\tvolumeEnabled := PumpHighVolumeRadarEnabled()
\tif !cfg.Enabled && !volumeEnabled {
\t\treturn func() {}
\t}
\tif db == nil {
\t\tlog.Printf("pump selective radar disabled: database unavailable")
\t\treturn func() {}
\t}
\tworkerCtx, cancel := context.WithCancel(ctx)
\tstore := NewSecurityRadarStore(db)
\tif cfg.Enabled {
\t\tadapter := NewPumpPortalRadarAdapter(store)
\t\tclient := NewPumpPortalClient(cfg)
\t\tgo client.Start(workerCtx, adapter.HandleEvent)
\t\tlog.Printf("pumpportal discovery started: free new-token/migration websocket=%s", cfg.redactedWebsocketHost())
\t}
\tif volumeEnabled {
\t\tworker := NewPumpHighVolumeRadarWorker(store, nil)
\t\tgo worker.Start(workerCtx)
\t\tlog.Printf("pump selective automatic radar started: volume_window=24h currency=USD threshold=%.0f poll=%s max_reports_per_cycle=%d rpc_saver=%t", worker.ThresholdUSD, worker.PollEvery, worker.MaxReportsPerCycle, SolanaRPCLimitSaverEnabled())
\t}
\treturn cancel
}'''
text = replace_once(text, old_start, new_start, "PumpPortal selective worker start")
path.write_text(text)

# Start the free Pump discovery and selective volume gate even while the broad
# Solana firehose remains paused by quota saver mode.
path = Path("main.go")
text = path.read_text()
old = '''\tif services.SolanaRPCLimitSaverEnabled() && !services.ForceBackgroundRadarEnabled() {
\t\tlog.Printf("background Solana streams paused: RPC saver mode protects Alchemy quota; manual scans and Safe Check stay live")
\t} else {
\t\tstopSBX1Stream := services.StartSecurityRadarStreamIfEnabled(appCtx, conn)
\t\tdefer stopSBX1Stream()
\t\tstopPumpPortal := services.StartPumpPortalRadarIfEnabled(appCtx, conn)
\t\tdefer stopPumpPortal()
\t}'''
new = '''\t// Pump discovery + the 500K USD selective gate are data-filtered and stay
\t// live in saver mode. Only mints crossing the gate consume deep Solana RPC.
\tstopPumpPortal := services.StartPumpPortalRadarIfEnabled(appCtx, conn)
\tdefer stopPumpPortal()
\tif services.SolanaRPCLimitSaverEnabled() && !services.ForceBackgroundRadarEnabled() {
\t\tlog.Printf("broad Solana streams paused: RPC saver protects quota; selective Pump 24h-volume radar, manual scans and Safe Check stay live")
\t} else {
\t\tstopSBX1Stream := services.StartSecurityRadarStreamIfEnabled(appCtx, conn)
\t\tdefer stopSBX1Stream()
\t}'''
text = replace_once(text, old, new, "main selective Pump startup")
path.write_text(text)

# Make watcher log accurate; the selective Pump worker is managed separately.
path = Path("internal/services/security_radar_watcher.go")
text = path.read_text()
text = replace_once(
    text,
    'log.Printf("security radar background workers paused: SOLANA_RPC_LIMIT_SAVER_ENABLED=true; manual Safe Check, token scans and user-triggered reports remain available")',
    'log.Printf("broad security radar RPC workers paused: SOLANA_RPC_LIMIT_SAVER_ENABLED=true; selective Pump volume radar is managed separately; manual scans remain available")',
    "watcher saver log",
)
path.write_text(text)

# Owner status, automatic 500K feed, and volume context in the human narrative.
path = Path("internal/handlers/owner_operations.go")
text = path.read_text()
old_status = '''\tradar := h.securityRadarStreamStats(ctx)
\tradarStatus := firstMapString(radar, "pipeline_status")
\tif strings.EqualFold(strings.TrimSpace(os.Getenv("SOLANA_RPC_LIMIT_SAVER_ENABLED")), "true") {
\t\tradarStatus = "manual_rpc_saver"
\t\tradar["pipeline_status"] = radarStatus
\t\tradar["background_streams_paused"] = true
\t\tradar["manual_scans_available"] = true
\t}'''
new_status = '''\tradar := h.securityRadarStreamStats(ctx)
\tradarStatus := firstMapString(radar, "pipeline_status")
\tif strings.EqualFold(strings.TrimSpace(os.Getenv("SOLANA_RPC_LIMIT_SAVER_ENABLED")), "true") {
\t\tradar["background_streams_paused"] = true
\t\tradar["manual_scans_available"] = true
\t\tif services.PumpHighVolumeRadarEnabled() {
\t\t\tradarStatus = "selective_auto_volume"
\t\t\tradar["pump_volume_auto_enabled"] = true
\t\t\tradar["pump_volume_threshold_usd"] = services.PumpHighVolumeThresholdUSD()
\t\t} else {
\t\t\tradarStatus = "manual_rpc_saver"
\t\t}
\t\tradar["pipeline_status"] = radarStatus
\t}'''
text = replace_once(text, old_status, new_status, "owner operations selective status")
old_overview = '''\titems := []services.SecurityRadarVerdictRecord{}
\tsources := []services.SecurityRadarSource{}
\tif db != nil {
\t\tstore := services.NewSecurityRadarStore(db)
\t\tif loaded, err := store.LatestVerdicts(r.Context(), 100); err == nil {
\t\t\titems = loaded
\t\t}
\t\tif loaded, err := store.ListSources(r.Context()); err == nil {
\t\t\tsources = loaded
\t\t}
\t}
\twriteJSON(w, http.StatusOK, map[string]any{
\t\t"ok": true, "generated_at": time.Now().UTC(), "items": items,
\t\t"sources": sources, "pipeline": h.securityRadarStreamStats(r.Context()),
\t})'''
new_overview = '''\titems := []services.SecurityRadarVerdictRecord{}
\tsources := []services.SecurityRadarSource{}
\thighVolumePump := []services.PumpHighVolumeOwnerItem{}
\tif db != nil {
\t\tstore := services.NewSecurityRadarStore(db)
\t\tif loaded, err := store.LatestVerdicts(r.Context(), 100); err == nil {
\t\t\titems = loaded
\t\t}
\t\tif loaded, err := store.ListSources(r.Context()); err == nil {
\t\t\tsources = loaded
\t\t}
\t\tif loaded, err := store.LatestPumpHighVolumeReports(r.Context(), 100); err == nil {
\t\t\thighVolumePump = loaded
\t\t}
\t}
\twriteJSON(w, http.StatusOK, map[string]any{
\t\t"ok": true, "generated_at": time.Now().UTC(), "items": items,
\t\t"high_volume_pump": highVolumePump,
\t\t"sources": sources, "pipeline": h.securityRadarStreamStats(r.Context()),
\t})'''
text = replace_once(text, old_overview, new_overview, "owner high-volume overview")
intro = '''\tparts := []string{
\t\tfmt.Sprintf("Koschei bu tokenı (%s) %.0f/100 ile %s risk seviyesinde değerlendiriyor. %s", ownerRadarShortTarget(target), risk, ownerRadarRiskLabelTR(level), ownerRadarRiskMeaning(level)),
\t}
'''
intro_new = '''\tparts := []string{
\t\tfmt.Sprintf("Koschei bu tokenı (%s) %.0f/100 ile %s risk seviyesinde değerlendiriyor. %s", ownerRadarShortTarget(target), risk, ownerRadarRiskLabelTR(level), ownerRadarRiskMeaning(level)),
\t}
\tif sourceSignals, ok := source["signals"].(map[string]any); ok {
\t\tvolume := radarDetailNumber(sourceSignals["volume_24h_usd"])
\t\tthreshold := radarDetailNumber(sourceSignals["volume_threshold_usd"])
\t\tif volume > 0 && threshold > 0 {
\t\t\tparts = append(parts, fmt.Sprintf("Bu rapor otomatik Pump hacim radarı tarafından açıldı: toplam 24 saatlik işlem hacmi $%.0f ve eşik $%.0f. Hacim tek başına güvenlik veya dolandırıcılık hükmü değildir; yalnızca derin inceleme tetikleyicisidir.", volume, threshold))
\t\t}
\t}
'''
text = replace_once(text, intro, intro_new, "owner volume narrative")
path.write_text(text)

# Prefer the high-volume gate event as the owner source context when present.
path = Path("internal/handlers/security_radar_detail.go")
text = path.read_text()
text = replace_once(
    text,
    "ORDER BY CASE WHEN source='pumpportal' THEN 0 ELSE 1 END, created_at DESC",
    "ORDER BY CASE WHEN event_type='pumpportal_high_volume_24h' THEN 0 WHEN source='pumpportal' THEN 1 ELSE 2 END, created_at DESC",
    "source context volume priority",
)
old_source = '''\tout["creator_wallet"] = creator
\tout["creator_label"] = "source-reported creator/deployer wallet"
\tout["creator_relation_verified"] = creator != "" && strings.EqualFold(source, "pumpportal")
\tout["creator_scope"] = "launch-source relation only; not proof of fraud, ownership of other wallets, or real-world identity"
\tout["signals"] = signals'''
new_source = '''\tout["creator_wallet"] = creator
\tout["creator_label"] = "source-reported creator/deployer wallet"
\tout["creator_relation_verified"] = creator != "" && (strings.EqualFold(source, "pumpportal") || strings.EqualFold(eventType, "pumpportal_high_volume_24h"))
\tout["creator_scope"] = "launch-source relation only; not proof of fraud, ownership of other wallets, or real-world identity"
\tout["volume_24h_usd"] = signals["volume_24h_usd"]
\tout["volume_threshold_usd"] = signals["volume_threshold_usd"]
\tout["volume_provider"] = signals["volume_provider"]
\tout["signals"] = signals'''
text = replace_once(text, old_source, new_source, "source context volume fields")
path.write_text(text)

# Pipeline metrics expose selective automatic operation rather than MANUAL.
path = Path("internal/handlers/security_radar.go")
text = path.read_text()
old_tail = '''\th.addArvisProcessingMetrics(ctx, metrics)
\tmetrics["pipeline_status"] = arvisPipelineStatus(metrics)
\treturn metrics
}'''
new_tail = '''\th.addArvisProcessingMetrics(ctx, metrics)
\tcount("pump_volume_reports_24h", `SELECT count(*) FROM security_radar_verdicts WHERE module_id='final_verdict_engine' AND signed=true AND source='pump_volume_gate' AND created_at > now()-interval '24 hours'`)
\tcount("pump_volume_qualified_24h", `SELECT count(DISTINCT target) FROM security_radar_events WHERE event_type='pumpportal_high_volume_24h' AND created_at > now()-interval '24 hours'`)
\ttext("pump_volume_last_observed_at", `SELECT COALESCE(max(created_at)::text,'') FROM security_radar_events WHERE event_type='pumpportal_high_volume_24h'`)
\tmetrics["pump_volume_auto_enabled"] = services.PumpHighVolumeRadarEnabled()
\tmetrics["pump_volume_threshold_usd"] = services.PumpHighVolumeThresholdUSD()
\tmetrics["pump_volume_window"] = "24h"
\tmetrics["pump_volume_currency"] = "USD"
\tstatus := arvisPipelineStatus(metrics)
\tif services.PumpHighVolumeRadarEnabled() && services.SolanaRPCLimitSaverEnabled() && !services.ForceBackgroundRadarEnabled() {
\t\tstatus = "selective_auto_volume"
\t}
\tmetrics["pipeline_status"] = status
\treturn metrics
}'''
text = replace_once(text, old_tail, new_tail, "selective pipeline metrics")
path.write_text(text)

# Owner frontend: dedicated 500K+ automatic report feed and cache-safe status.
path = Path("public/js/owner-control-center.js")
text = path.read_text()
text = replace_once(
    text,
    "'manual rpc saver':'Manuel · RPC Tasarrufu'",
    "'manual rpc saver':'Manuel · RPC Tasarrufu','selective auto volume':'Otomatik · Pump 500K+','evidence pending':'Kanıt Bekliyor'",
    "owner status labels",
)
pattern = re.compile(r"function renderArvis\(\)\{.*?\}\nfunction radarFeedTable\(items\)\{.*?\}\nfunction sourceList", re.S)
replacement = r'''function renderArvis(){const d=state.arvis||{},p=obj(d.pipeline),items=arr(d.items),auto=arr(d.high_volume_pump),sources=arr(d.sources),threshold=Number(p.pump_volume_threshold_usd||500000);$('arvisContent').innerHTML=`<div class="grid compact-grid"><article class="card span-12"><div class="card-head"><div><span class="eyebrow">Owner full scan</span><h2>Mint / cüzdan / program / işlem tara</h2></div>${badge(p.pipeline_status||'unknown')}</div><form id="ownerRadarForm" class="filters" style="display:grid;grid-template-columns:minmax(0,1fr) auto;gap:9px"><input class="input mono" id="ownerRadarTarget" placeholder="Solana adresi veya mint"><button class="btn primary" id="ownerRadarRun" type="submit">Tam Tara + Görsel Üret</button></form><div id="ownerRadarResult" class="section-gap"><div class="empty">Adres girildiğinde bütün doğrulanmış modüller, holder dağılımı, creator/deployer ilişkisi, graph ve kanıtlar burada açılır.</div></div></article><article class="card span-12"><div class="card-head"><div><span class="eyebrow">Otomatik Pump hacim radarı</span><h2>24 saatte $${num(threshold)}+ işlem hacmi</h2><p class="muted">PumpPortal keşfi → DexScreener 24s USD hacmi → tam ARVIS raporu. Eşik altındaki token RPC tüketmez.</p></div><span class="badge ok">${num(auto.length)} token</span></div>${pumpVolumeAutoTable(auto)}</article><article class="card span-8"><div class="card-head"><div><span class="eyebrow">Son kararlar</span><h2>Tek token · tek doğrulanmış hikâye</h2></div><span class="badge ok">${num(items.length)} kart</span></div>${radarFeedTable(items)}</article><article class="card span-4"><div class="card-head"><div><span class="eyebrow">Kaynak kayıtları</span><h2>Doğrulanmış gözlemciler</h2></div></div>${sourceList(sources)}</article></div>`;$('ownerRadarForm').onsubmit=async e=>{e.preventDefault();const target=$('ownerRadarTarget').value.trim();if(!target)return toast('Adres gerekli.',true);await OwnerRadarKit.scan(target,'ownerRadarResult')};document.querySelectorAll('[data-scan-target]').forEach(b=>b.onclick=()=>{const target=b.dataset.scanTarget;$('ownerRadarTarget').value=target;OwnerRadarKit.scan(target,'ownerRadarResult')})}
function pumpVolumeAutoTable(items){if(!items.length)return'<div class="empty">Henüz 24 saatlik hacmi $500.000 eşiğini geçen Pump token gözlemlenmedi.</div>';return`<div class="table-wrap"><table class="table"><thead><tr><th>Token</th><th>24s hacim</th><th>Likidite</th><th>Rapor</th><th>Risk</th><th>Zaman</th><th></th></tr></thead><tbody>${items.map(x=>`<tr><td><b>${esc(x.symbol||x.name||'Pump token')}</b><div class="mono">${esc(short(x.target,32))}</div></td><td><b>$${num(x.volume_24h_usd)}</b><div class="muted small">${num(x.pair_count)} pair · ${esc(x.volume_provider||'dexscreener')}</div></td><td>$${num(x.liquidity_usd)}</td><td>${badge(x.report_status||'evidence_pending')}</td><td>${x.risk_index==null?'N/A':`<b>${num(x.risk_index)}/100</b><br>${badge(x.risk_level)}`}</td><td>${dt(x.observed_at)}</td><td><button class="btn small" data-scan-target="${esc(x.target)}" type="button">Tam rapor</button></td></tr>`).join('')}</tbody></table></div>`}
function radarFeedTable(items){if(!items.length)return'<div class="empty">Henüz doğrulanmış Radar kararı yok.</div>';return`<div class="table-wrap"><table class="table"><thead><tr><th>Hedef</th><th>Risk</th><th>Karar</th><th>Kaynak</th><th>Zaman</th><th></th></tr></thead><tbody>${items.slice(0,50).map(x=>{const s=obj(x.signals),auto=s.auto_volume_gate===true,source=auto?`AUTO · $${num(s.volume_24h_usd)}`:(x.source||x.provider||'ARVIS');return`<tr><td class="mono">${esc(short(x.target,34))}${auto?'<br><span class="badge ok">PUMP 500K+</span>':''}</td><td><b>${num(x.risk_index)}/100</b><br>${badge(x.risk_level)}</td><td>${esc(short(x.verdict,80))}</td><td>${esc(source)}</td><td>${dt(x.created_at)}</td><td><button class="btn small" data-scan-target="${esc(x.target)}" type="button">Aç</button></td></tr>`}).join('')}</tbody></table></div>`}
function sourceList'''
text, count = pattern.subn(replacement, text, count=1)
if count != 1:
    raise SystemExit("owner ARVIS functions replacement failed")
path.write_text(text)

path = Path("public/owner-production.html")
text = path.read_text()
text = replace_once(text, '/js/owner-control-center.js?v=7', '/js/owner-control-center.js?v=8', "owner cache v8")
path.write_text(text)

# Production configuration contract.
path = Path("../../.env.example")
text = path.read_text()
anchor = '''RAYDIUM_PROGRAM_ID=675kPX9MHTjS2zt1qfr1NYHuzeLXfQM9H24wFSUt1Mp8
PUMP_FUN_PROGRAM_ID=6EF8rrecthR5Dkzon8Nwu78hRvfCKubJ14M5uBEwF6P
'''
addition = '''RAYDIUM_PROGRAM_ID=675kPX9MHTjS2zt1qfr1NYHuzeLXfQM9H24wFSUt1Mp8
PUMP_FUN_PROGRAM_ID=6EF8rrecthR5Dkzon8Nwu78hRvfCKubJ14M5uBEwF6P

# Selective automatic Pump Radar. Free PumpPortal discovery stays live in RPC saver mode;
# only mints whose aggregated 24h USD pair volume crosses the threshold receive full ARVIS RPC enrichment.
PUMPPORTAL_ENABLED=true
PUMPPORTAL_DATA_WS=wss://pumpportal.fun/api/data
PUMPPORTAL_API_KEY=
PUMP_HIGH_VOLUME_RADAR_ENABLED=true
PUMP_HIGH_VOLUME_MIN_24H_USD=500000
PUMP_HIGH_VOLUME_POLL_SECONDS=300
PUMP_HIGH_VOLUME_REPORT_COOLDOWN_SECONDS=21600
PUMP_HIGH_VOLUME_CANDIDATE_PAGE_SIZE=900
PUMP_HIGH_VOLUME_MAX_REPORTS_PER_CYCLE=3
'''
text = replace_once(text, anchor, addition, "selective Pump env contract")
path.write_text(text)
