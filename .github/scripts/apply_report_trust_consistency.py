from pathlib import Path


def replace_once(path: str, old: str, new: str) -> None:
    file = Path(path)
    text = file.read_text()
    if old not in text:
        raise RuntimeError(f"marker not found in {path}: {old[:160]!r}")
    file.write_text(text.replace(old, new, 1))


# 1) Jupiter's priceImpactPct is a 0..1 ratio. Store a real percentage.
replace_once(
    "koschei/api/internal/handlers/lp_market_context.go",
    '\t\t\t\t\t\tout.EstimatedPriceImpactPct, _ = strconv.ParseFloat(strings.TrimSpace(quote.PriceImpactPct), 64)\n',
    '''\t\t\t\t\t\timpactRatio, parseImpactErr := strconv.ParseFloat(strings.TrimSpace(quote.PriceImpactPct), 64)
\t\t\t\t\t\tif parseImpactErr == nil {
\t\t\t\t\t\t\tout.EstimatedPriceImpactPct = roundCollectorPct(math.Max(0, math.Min(1, impactRatio)) * 100)
\t\t\t\t\t\t}
''',
)
replace_once(
    "koschei/api/internal/handlers/lp_market_context_test.go",
    '`{"outAmount":"90000000","priceImpactPct":"12.5","contextSlot":889,"routePlan":[{"swapInfo":{"label":"Raydium CPMM"}}]}`',
    '`{"outAmount":"90000000","priceImpactPct":"0.125","contextSlot":889,"routePlan":[{"swapInfo":{"label":"Raydium CPMM"}}]}`',
)

# 2) Safe Check must expose what it did not investigate and must not imply an
# investment-grade token verdict for an address-shaped target.
preflight = "koschei/api/internal/handlers/arvis_preflight.go"
replace_once(
    preflight,
    '''\tAIExplanation  string   `json:"ai_explanation,omitempty"`
\tCreditsCharged bool     `json:"credits_charged"`
''',
    '''\tAIExplanation  string   `json:"ai_explanation,omitempty"`
\tCreditsCharged bool     `json:"credits_charged"`
\tScope          string   `json:"scope"`
\tCoverageWarning string  `json:"coverage_warning,omitempty"`
\tNotChecked     []string `json:"not_checked"`
''',
)
replace_once(
    preflight,
    '''\tresp = h.alignARVISPreflightWithStructuralBaseline(r.Context(), req, resp)
\tif aiProviderConfigured()''',
    '''\tresp = h.alignARVISPreflightWithStructuralBaseline(r.Context(), req, resp)
\tresp = applyARVISPreflightScope(req, resp)
\tif aiProviderConfigured()''',
)
replace_once(
    preflight,
    '''func applyARVISStructuralBaseline(resp arvisPreflightResponse, floor int, level string, observedAt time.Time) arvisPreflightResponse {''',
    '''func applyARVISPreflightScope(req arvisPreflightRequest, resp arvisPreflightResponse) arvisPreflightResponse {
\tresp.Scope = "preflight_only"
\tresp.NotChecked = []string{}
\ttarget := strings.TrimSpace(req.Target)
\tif !solanaPreflightAddressLike.MatchString(target) {
\t\treturn resp
\t}
\tresp.NotChecked = []string{
\t\t"Owner-resolved holder dağılımı",
\t\t"LP sahipliği, yakım, kilit ve unlock koşulları",
\t\t"Creator geçmişi, funding ve bağlantılı cüzdanlar",
\t\t"Canlı satış, çıkış ve likidite hareketleri",
\t}
\tresp.CoverageWarning = "Bu hızlı ön kontrol holder dağılımını, LP kontrolünü, creator geçmişini ve canlı çıkış kapasitesini çalıştırmaz. Bu skor yatırım güvenliği hükmü değildir."
\tif resp.Decision == "allow" {
\t\tresp.Decision = "review"
\t}
\tif resp.RiskLevel == "low" {
\t\tresp.RiskLevel = "unknown"
\t}
\tif !strings.Contains(resp.HumanMessage, "yatırım güvenliği hükmü değildir") {
\t\tresp.HumanMessage = strings.TrimSpace(resp.HumanMessage + " Holder ve likidite kapsamı çalıştırılmadı; bu sonuç yatırım güvenliği hükmü değildir.")
\t}
\treturn resp
}

func applyARVISStructuralBaseline(resp arvisPreflightResponse, floor int, level string, observedAt time.Time) arvisPreflightResponse {''',
)

preflight_test = "koschei/api/internal/handlers/arvis_preflight_test.go"
replace_once(
    preflight_test,
    '''func TestARVISPreflightRaisesRiskForGuarantees(t *testing.T) {''',
    '''func TestARVISPreflightMarksAddressCoverageIncomplete(t *testing.T) {
\treq := arvisPreflightRequest{Target: "So11111111111111111111111111111111111111112", Kind: "token", Intent: "buy"}
\tgot := applyARVISPreflightScope(req, evaluateARVISPreflight(req))
\tif got.Scope != "preflight_only" || got.CoverageWarning == "" || len(got.NotChecked) < 4 {
\t\tt.Fatalf("coverage contract missing: %+v", got)
\t}
\tif got.Decision == "allow" || got.RiskLevel == "low" {
\t\tt.Fatalf("address preflight implied safety: %+v", got)
\t}
}

func TestARVISPreflightRaisesRiskForGuarantees(t *testing.T) {''',
)

# 3) Publish an all-time signed verdict metric instead of presenting a recent
# zero beside the all-time completed-processing counter.
replace_once(
    "koschei/api/internal/handlers/security_radar.go",
    '''\tcount("visible_verdicts", `SELECT count(*) FROM security_radar_verdicts WHERE module_id='final_verdict_engine' AND signed=true AND created_at > now() - interval '24 hours' AND `+verifiedSQL)
''',
    '''\tcount("visible_verdicts", `SELECT count(*) FROM security_radar_verdicts WHERE module_id='final_verdict_engine' AND signed=true AND created_at > now() - interval '24 hours' AND `+verifiedSQL)
\tcount("signed_verdicts_total", `SELECT count(*) FROM security_radar_verdicts WHERE module_id='final_verdict_engine' AND signed=true AND `+verifiedSQL)
''',
)
replace_once(
    "koschei/api/internal/handlers/health.go",
    '''\t\t"visible_verdicts":        stats["visible_verdicts"],
''',
    '''\t\t"visible_verdicts":        stats["visible_verdicts"],
\t\t"signed_verdicts_total":   stats["signed_verdicts_total"],
''',
)

# 4) Landing and Safe Check render the coverage boundary explicitly.
index = "koschei/api/public/index.html"
replace_once(
    index,
    "function render(data){const score=data.score??data.risk_index??0,level=data.risk_level||'unknown',decision=data.decision||data.policy||'review',k=kind(score,level,decision),reasons=Array.isArray(data.reasons)?data.reasons:[],steps=Array.isArray(data.next_steps)?data.next_steps:[];result.className='result show';result.innerHTML='<div class=\"score '+k+'\">'+esc(score)+'</div><b>'+decisionLabel(decision)+' · '+esc(levelLabel(level))+'</b><p class=\"sub\" style=\"margin-top:6px\">'+esc(data.human_message||data.verdict||'ARVIS ön kontrolü tamamlandı.')+'</p>'+reasons.concat(steps).slice(0,5).map(x=>'<div class=\"line\">'+esc(x)+'</div>').join('')+'<div class=\"actions\" style=\"margin-top:12px\"><a class=\"btn primary\" href=\"/safe-check\">Ayrıntılı kontrol</a><a class=\"btn\" href=\"/security-radar?target='+encodeURIComponent(target.value.trim())+'\">Geniş tarama</a></div>'}",
    "function render(data){const score=data.score??data.risk_index??0,level=data.risk_level||'unknown',decision=data.decision||data.policy||'review',limited=Boolean(data.coverage_warning),k=limited?'warn':kind(score,level,decision),reasons=Array.isArray(data.reasons)?data.reasons:[],steps=Array.isArray(data.next_steps)?data.next_steps:[],missing=Array.isArray(data.not_checked)?data.not_checked:[];result.className='result show';result.innerHTML='<div class=\"score '+k+'\">'+esc(score)+'<small style=\"display:block;font-size:10px\">PRECHECK / 100</small></div><b>'+decisionLabel(decision)+' · '+esc(levelLabel(level))+'</b><p class=\"sub\" style=\"margin-top:6px\">'+esc(data.human_message||data.verdict||'ARVIS ön kontrolü tamamlandı.')+'</p>'+(limited?'<div class=\"line\"><b>KAPSAM SINIRI</b><br>'+esc(data.coverage_warning)+(missing.length?'<br><small>Kontrol edilmedi: '+missing.map(esc).join(' · ')+'</small>':'')+'</div>':'')+reasons.concat(steps).slice(0,5).map(x=>'<div class=\"line\">'+esc(x)+'</div>').join('')+'<div class=\"actions\" style=\"margin-top:12px\"><a class=\"btn primary\" href=\"/safe-check\">Ayrıntılı kontrol</a><a class=\"btn\" href=\"/security-radar?target='+encodeURIComponent(target.value.trim())+'\">Geniş tarama</a></div>'}",
)
replace_once(
    index,
    "document.getElementById('verdicts').textContent=tr(a.visible_verdicts||a.processing_completed_recent);",
    "const signed=Number(a.signed_verdicts_total??a.visible_verdicts??0);document.getElementById('verdicts').textContent=signed>0?tr(signed):'—';",
)

safe = "koschei/api/public/safe-check.html"
replace_once(
    safe,
    "function render(data,rawTarget){const score=data.score??data.risk_index??0,level=data.risk_level||'unknown',decision=data.decision||data.policy||'review',kind=cls(score,level,decision),reasons=Array.isArray(data.reasons)?data.reasons:[],steps=Array.isArray(data.next_steps)?data.next_steps:[];out.innerHTML=`<div class=\"score ${kind}\"><div><strong>${esc(score)}</strong><div class=\"muted\">Basic preflight risk</div></div></div><div class=\"decision\">${label(decision)} · ${esc(level)}</div><p class=\"muted\" style=\"margin-top:8px\">${esc(data.human_message||data.verdict||'ARVIS preflight tamamlandı.')}</p><div class=\"list\">${reasons.concat(steps).slice(0,8).map(x=>`<div class=\"item\">${esc(x)}</div>`).join('')||'<div class=\"item\">Ek temel sinyal yok.</div>'}</div><div class=\"upgrade\"><b>Bu yalnız ücretsiz temel kontroldür.</b><p>Creator/deployer, owner-normalized holder, liquidity control, threat pathways, graph ve bütün kanıt açıklamaları için Tam Radar’ı aç.</p><a class=\"btn\" href=\"${esc(radarLink(rawTarget))}\">Tam Security Radar Raporu →</a></div>`}",
    "function render(data,rawTarget){const score=data.score??data.risk_index??0,level=data.risk_level||'unknown',decision=data.decision||data.policy||'review',limited=Boolean(data.coverage_warning),kind=limited?'warn':cls(score,level,decision),reasons=Array.isArray(data.reasons)?data.reasons:[],steps=Array.isArray(data.next_steps)?data.next_steps:[],missing=Array.isArray(data.not_checked)?data.not_checked:[];out.innerHTML=`<div class=\"score ${kind}\"><div><strong>ÖN</strong><div class=\"muted\">Hızlı preflight · ${esc(score)}/100</div></div></div><div class=\"decision\">${label(decision)} · ${esc(level)}</div><p class=\"muted\" style=\"margin-top:8px\">${esc(data.human_message||data.verdict||'ARVIS preflight tamamlandı.')}</p>${limited?`<div class=\"upgrade\"><b>Holder ve likidite bu sonuçta değerlendirilmedi.</b><p>${esc(data.coverage_warning)}</p>${missing.length?`<p class=\"fine\">Kontrol edilmedi: ${missing.map(esc).join(' · ')}</p>`:''}</div>`:''}<div class=\"list\">${reasons.concat(steps).slice(0,8).map(x=>`<div class=\"item\">${esc(x)}</div>`).join('')||'<div class=\"item\">Ek temel sinyal yok.</div>'}</div><div class=\"upgrade\"><b>Bu yalnız ücretsiz temel kontroldür.</b><p>Creator/deployer, owner-normalized holder, liquidity control, threat pathways, graph ve bütün kanıt açıklamaları için Tam Radar’ı aç.</p><a class=\"btn\" href=\"${esc(radarLink(rawTarget))}\">Tam Security Radar Raporu →</a></div>`}",
)

# 5) Rewrite the public scan renderer so pending/window signals are grouped and
# a preflight never renders an A-F investment grade.
Path("koschei/api/public/js/public-solana-scan.js").write_text(r'''(()=>{
'use strict';
const OFFICIAL_KOSCH_MINT='HHPpU9u56Bwxov12nf7DXUCuv6h1q5j1xgGS3yukpump';
const $=id=>document.getElementById(id),esc=value=>String(value??'').replace(/[&<>"']/g,ch=>({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[ch]));
const form=$('scanForm'),submit=$('submit'),target=$('target'),kind=$('kind'),note=$('note'),empty=$('empty'),result=$('result'),share=$('shareResult'),openExplorer=$('openExplorer');
const requestGuard=window.KoscheiPublicScanGuard;
let lastShareURL='';
const clamp=n=>Math.max(0,Math.min(100,Math.round(Number(n)||0)));
const level=r=>r>=85?'critical':r>=65?'high':r>=35?'medium':'low';
const short=value=>{const text=String(value??'');return text.length>24?`${text.slice(0,10)}…${text.slice(-8)}`:text};
function fetchJSON(url,options){return fetch(url,options).then(async response=>{const data=await response.json().catch(()=>({}));if(!response.ok)throw new Error(data.error||'scan_failed');return data})}
function stateLabel(state){return({verified:'DOĞRULANDI',observed:'GÖZLENDİ',window_open:'İZLEME PENCERESİ',not_applicable:'UYGULANAMAZ',arm_pending:'KANIT KOLU EKSİK'}[state]||String(state||'').toUpperCase())}
function refChip(type,value){const raw=String(value??'').trim();if(!raw)return'';return`<button class="evidence-ref" type="button" data-copy-ref="${esc(raw)}" title="Kopyala: ${esc(raw)}"><span>${esc(type)}</span><b>${esc(short(raw))}</b></button>`}
function renderRefs(refs={}){const chips=[];(Array.isArray(refs.wallets)?refs.wallets:[]).forEach(value=>chips.push(refChip('wallet',value)));(Array.isArray(refs.accounts)?refs.accounts:[]).forEach(value=>chips.push(refChip('account',value)));(Array.isArray(refs.signatures)?refs.signatures:[]).forEach(value=>chips.push(refChip('signature',value)));(Array.isArray(refs.slots)?refs.slots:[]).forEach(value=>chips.push(refChip('slot',value)));(Array.isArray(refs.evidence_keys)?refs.evidence_keys:[]).forEach(value=>chips.push(refChip('evidence',value)));return chips.length?`<div class="evidence-refs" aria-label="Kanıt referansları">${chips.join('')}</div>`:''}
function signalRows(rows){return rows.map(row=>`<div class="public-signal ${esc(row.state)}" id="evidence-${esc(row.id)}"><span><b>${esc(row.label)}</b><small>${esc(stateLabel(row.state))}</small>${row.detail?`<small>${esc(row.detail)}</small>`:''}</span><em>${esc(row.value)}</em>${renderRefs(row.refs)}</div>`).join('')}
function renderTechnicalReport(report,mint){
  if(!report||!window.KoscheiVerdictCard)return false;
  const vm=window.KoscheiVerdictCard.mapVerdictCard(report,{lang:'tr'}),h=vm.header;
  const checklist=Array.isArray(vm.checklist)?vm.checklist:[];
  const pending=checklist.filter(row=>['arm_pending','window_open'].includes(row.state));
  const resolved=checklist.filter(row=>!['arm_pending','window_open'].includes(row.state));
  const lpHTML=window.KoscheiLPControlCard?.render(report,{lang:'tr'})||'';
  const liveHTML=window.KoscheiLiveEvidenceCard?.render(report,{lang:'tr'})||'';
  empty.hidden=true;result.hidden=false;
  result.innerHTML=`<article class="public-investigation-card"><div class="resultHead"><div class="grade">${esc(h.grade||h.icon||'✓')}</div><div><div class="risk">${esc(h.title)}</div><div class="badge medium">İMZALI TEKNİK RAPOR</div></div></div><p class="sub" style="margin-top:16px">${esc(h.copy)}</p><div class="target">${esc(report.target||mint)}</div><div class="verdictMeta" data-signed="${h.signature_short?'true':'false'}">Ruleset ${esc(h.ruleset_version)} · imza ${esc(h.signature_short||'bekliyor')} · ${esc(h.generated_at||'')}</div><div class="official" ${mint===OFFICIAL_KOSCH_MINT?'':'hidden'}><strong>Resmî KOSCH mint eşleşti.</strong><br>Bu etiket yalnız varlık kimliğini doğrular.</div><div class="section"><h3>Kanıt kapsamı</h3><p class="historySummary">${esc(vm.coverage.text)}</p></div><div class="section"><h3>Doğrulanmış, gözlenen ve uygulanamaz sinyaller</h3><div class="public-signal-list">${signalRows(resolved)||'<div class="public-signal"><span><b>Tamamlanan sinyal yok</b></span></div>'}</div></div>${pending.length?`<div class="section"><h3>Bekleyen kanıt kolları ve izleme pencereleri (${pending.length})</h3><p class="historySummary">Bu satırlar nihai hükmü güvenli seviyeye yükseltmez; yalnız tamamlanmayan veya zamana bağlı kanıtı gösterir.</p><div class="public-signal-list">${signalRows(pending)}</div></div>`:''}<div class="section"><h3>${esc(vm.leverage_title)}</h3>${vm.leverage.length?`<ul class="list">${vm.leverage.map(row=>`<li>${esc(row.text)}</li>`).join('')}</ul>`:'<p class="historySummary">Doğrulanmış aktif kontrol satırı gözlenmedi; bu ifade risksiz anlamına gelmez.</p>'}</div><p class="fine">${esc(vm.disclaimer)}</p></article>${lpHTML}${liveHTML}`;
  lastShareURL=`${location.origin}/scan/${encodeURIComponent(report.target||mint)}`;share.hidden=false;openExplorer.hidden=false;openExplorer.href=`https://solscan.io/token/${encodeURIComponent(mint)}`;history.replaceState({},'',`/scan/${encodeURIComponent(mint)}`);return true;
}
function renderPreflight(data,value){const risk=clamp(data.score),limited=Boolean(data.coverage_warning),missing=Array.isArray(data.not_checked)?data.not_checked:[];empty.hidden=true;result.hidden=false;result.innerHTML=`<div class="resultHead"><div class="grade">ÖN</div><div><div class="risk">HIZLI ÖN KONTROL · ${esc(risk)}/100</div><div class="badge ${limited?'medium':esc(level(risk))}">${esc(String(data.decision||'review').toUpperCase())}</div></div></div><p class="sub" style="margin-top:16px">${esc(data.human_message||'Ön kontrol tamamlandı.')}</p><div class="target">${esc(value)}</div>${limited?`<div class="section"><h3>Kapsam sınırı</h3><p class="historySummary">${esc(data.coverage_warning)}</p>${missing.length?`<ul class="list">${missing.map(item=>`<li>${esc(item)}</li>`).join('')}</ul>`:''}</div>`:''}<div class="section"><h3>Gözlenen nedenler</h3><ul class="list">${(Array.isArray(data.reasons)?data.reasons:[]).slice(0,8).map(item=>`<li>${esc(item)}</li>`).join('')||'<li>Ön kontrol ek neden satırı üretmedi.</li>'}</ul></div><p class="fine">Safe Check hızlı preflight’tır; tam token araştırması veya yatırım güvenliği hükmü değildir.</p>`;lastShareURL=location.href;share.hidden=false;openExplorer.hidden=true}
async function runScan(){const value=target.value.trim();if(!value)return;const tokenScan=kind.value==='token';const requestToken=tokenScan&&requestGuard?requestGuard.begin(result,value):null;submit.disabled=true;submit.textContent='ARVIS kanıtları topluyor…';empty.hidden=false;empty.innerHTML='<h2>Teknik araştırma çalışıyor</h2><p>Collector sonuçları, holder kontrolü, canlı işlem penceresi ve piyasa bağlamı aynı raporda birleştiriliyor.</p>';result.hidden=true;share.hidden=true;openExplorer.hidden=true;try{if(tokenScan){const data=await fetchJSON('/api/token/scan',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({mint:value,network:'solana-mainnet'})});const report=data.investigation_report;const decision=requestToken?requestGuard.accept(requestToken,report):{accepted:true};if(!decision.accepted){if(decision.reason==='stale_response')return;throw new Error(`scan_target_mismatch:${decision.expected}:${decision.returned}`)}if(!renderTechnicalReport(report,value))throw new Error('investigation_report_missing')}else{const data=await fetchJSON('/api/arvis/preflight',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({target:value,kind:kind.value,intent:note.value.trim(),note:note.value.trim()})});renderPreflight(data,value)}}catch(error){if(requestToken&&requestGuard&&!requestGuard.isActive(requestToken))return;empty.hidden=false;empty.innerHTML=String(error?.message||'').startsWith('scan_target_mismatch:')?'<h2>Tarama hedefi uyuşmadı</h2><p>Eski veya farklı hedefe ait sonuç ekrana basılmadı.</p>':'<h2>Tarama tamamlanamadı</h2><p>Bu hedef için teknik rapor üretilemedi. İşlem yapmadan önce tekrar dene.</p>'}finally{const ownsUI=!requestToken||!requestGuard||requestGuard.isActive(requestToken);if(requestToken&&requestGuard)requestGuard.finish(requestToken);if(ownsUI){submit.disabled=false;submit.textContent='ARVIS taramasını başlat'}}}
form.addEventListener('submit',event=>{event.preventDefault();runScan()});
result.addEventListener('click',async event=>{const button=event.target.closest('[data-copy-ref]');if(!button)return;try{await navigator.clipboard.writeText(button.dataset.copyRef||'');const previous=button.innerHTML;button.textContent='Kopyalandı';setTimeout(()=>{button.innerHTML=previous},900)}catch{}});
share.addEventListener('click',async()=>{const payload={title:'Koschei ARVIS teknik araştırma raporu',text:'Koschei ARVIS teknik araştırma sonucu',url:lastShareURL};try{if(navigator.share)await navigator.share(payload);else{await navigator.clipboard.writeText(lastShareURL);share.textContent='Link kopyalandı'}}catch{}});
const params=new URLSearchParams(location.search),pathMint=location.pathname.startsWith('/scan/')?decodeURIComponent(location.pathname.slice(6).split('/')[0]||''):'';const initial=pathMint||params.get('mint')||params.get('target')||'';if(initial){target.value=initial;kind.value='token';runScan()}
})();
''')

# 6) Rewrite LP movement fallback copy: no naked failure counter in the main UI.
Path("koschei/api/public/js/lp-control-evidence-card.js").write_text(r'''(()=>{
  'use strict';
  const esc=value=>String(value??'').replace(/[&<>"']/g,char=>({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[char]));
  const obj=value=>value&&typeof value==='object'&&!Array.isArray(value)?value:{};
  const arr=value=>Array.isArray(value)?value:[];
  const text=value=>String(value??'').trim();
  const short=(value,length=36)=>{const raw=text(value);return raw.length>length?`${raw.slice(0,16)}…${raw.slice(-12)}`:raw||'—'};
  const number=(value,digits=8)=>Number.isFinite(Number(value))?new Intl.NumberFormat('en-US',{maximumFractionDigits:digits}).format(Number(value)):'—';
  const pct=value=>Number.isFinite(Number(value))?`${number(value,4)}%`:'—';
  const statusLabel=(value,lang)=>{const key=text(value).toLowerCase();const tr={burned:'LP YAKIMI GÖZLENDİ',locked_until:'SÜRELİ KİLİT DOĞRULANDI',permanently_locked:'KALICI KİLİT GÖZLENDİ',held_by_creator:'CREATOR LP PAYI GÖZLENDİ',unverified:'KONTROL SAHİBİ DOĞRULANMADI',observed:'GÖZLENDİ',complete_no_movement_observed:'PENCEREDE HAREKET YOK',partial_no_movement_observed:'KISMİ PENCERE',collection_failed:'HAREKET GEÇMİŞİ DOĞRULANAMADI',source_unavailable:'KAYNAK ALINAMADI',not_applicable:'UYGULANAMAZ'};const en={burned:'LP BURN OBSERVED',locked_until:'TIME LOCK VERIFIED',permanently_locked:'PERMANENT LOCK OBSERVED',held_by_creator:'CREATOR LP SHARE OBSERVED',unverified:'CONTROL OWNER UNVERIFIED',observed:'OBSERVED',complete_no_movement_observed:'NO MOVEMENT IN WINDOW',partial_no_movement_observed:'PARTIAL WINDOW',collection_failed:'MOVEMENT HISTORY UNVERIFIED',source_unavailable:'SOURCE UNAVAILABLE',not_applicable:'NOT APPLICABLE'};return(lang==='tr'?tr:en)[key]||key.replaceAll('_',' ').toUpperCase()||'UNKNOWN'};
  const copy=value=>`<button type="button" class="lp-ref" data-copy-ref="${esc(text(value))}" title="${esc(text(value))}">${esc(short(value))}</button>`;
  function movementRows(lp,lang){return arr(lp.liquidity_movements).map(row=>`<tr><td><b>${esc(statusLabel(row.kind,lang))}</b><small>${esc(text(row.verification_status)||'—')} · ${esc(text(row.block_time)||'—')}</small></td><td>${copy(row.actor_wallet)}<small>${esc(text(row.creator_relation)||'—')}</small></td><td>${esc(number(row.token_delta))}</td><td>${esc(number(row.quote_delta))}</td><td>${copy(row.signature)}<small>slot ${esc(row.slot||'—')}</small></td></tr>`).join('')}
  function movementFallback(lp,lang){const failed=Number(lp.movement_window_failures||0),parsed=Number(lp.movement_window_parsed||0),signatures=Number(lp.movement_window_signatures||0),status=text(lp.movement_status);const unavailable=failed>0||['source_unavailable','collection_failed'].includes(status);const title=lang==='tr'?(unavailable?'Havuz hareket geçmişi bu taramada doğrulanamadı':'Sınırlı pencerede add/remove hareketi gözlenmedi'):(unavailable?'Pool movement history could not be verified in this scan':'No add/remove movement was observed in the bounded window');const copyText=lang==='tr'?(unavailable?'Bu durum likidite hareketi olmadığı anlamına gelmez. Kaynak erişimi tamamlandığında pencere yeniden taranmalıdır.':'Bu sonuç yalnız incelenen son pencereyi kapsar; eski hareketlerin yokluğunu kanıtlamaz.'):(unavailable?'This does not mean no liquidity movement exists. Re-scan when the source is available.':'This covers only the bounded recent window and does not prove older movements do not exist.');return`<div class="lp-window-line"><b>${esc(title)}</b><span>${esc(copyText)}</span><details><summary>Teknik toplama ayrıntısı</summary><small>${esc(signatures)} imza görüldü · ${esc(parsed)} işlem ayrıştırıldı · ${esc(failed)} kaynak hatası</small></details></div>`}
  function render(payload,options={}){const lang=options.lang==='tr'?'tr':'en';const lp=obj(payload?.lp_control||payload?.investigation_report?.lp_control);if(!Object.keys(lp).length||(!text(lp.pool_address)&&text(lp.status)==='not_applicable'))return'';const position=text(lp.control_model)==='position_nft';const movements=movementRows(lp,lang);const title=lang==='tr'?'LİKİDİTE KONTROL KANITI':'LIQUIDITY CONTROL EVIDENCE';const snapshot=lang==='tr'?'POOL VE REZERV SNAPSHOT':'POOL AND RESERVE SNAPSHOT';const control=lang==='tr'?'KONTROL YÜZEYİ':'CONTROL SURFACE';const movementTitle=lang==='tr'?'ADD / REMOVE LİKİDİTE İŞLEMLERİ':'ADD / REMOVE LIQUIDITY TRANSACTIONS';return`<article class="card lp-control-card" id="lp-control-evidence"><div class="card-head"><div><span class="eyebrow">${title}</span><h2>${esc(text(lp.pool_type)||'Pool')}</h2><p class="muted">${esc(text(lp.control_model)||'unresolved')} · ${esc(text(lp.position_model)||'—')}</p></div><span class="badge ${['burned','locked_until','permanently_locked'].includes(text(lp.status))?'ok':'warn'}">${esc(statusLabel(lp.status,lang))}</span></div><div class="lp-address-grid"><div><label>Pool</label>${copy(lp.pool_address)}</div><div><label>Program</label>${copy(lp.pool_program)}</div><div><label>Read slot</label><b>${esc(lp.read_slot||'—')}</b></div><div><label>Canonical</label><b>${lp.canonical_pool?'YES':'—'}</b></div></div><h3>${snapshot}</h3><div class="lp-metrics"><div><label>Token vault</label>${copy(lp.token_vault)}<b>${esc(number(lp.token_reserve))}</b></div><div><label>Quote vault</label>${copy(lp.quote_vault)}<b>${esc(number(lp.quote_reserve))}</b></div>${Number(lp.virtual_quote_reserve)>0?`<div><label>Virtual quote reserve</label><b>${esc(number(lp.virtual_quote_reserve))}</b></div>`:''}<div><label>Reserve value</label><b>${Number(lp.reserve_liquidity_usd)>0?`$${esc(number(lp.reserve_liquidity_usd,2))}`:'—'}</b></div></div><h3>${control}</h3>${position?`<div class="lp-metrics"><div><label>Pool liquidity raw</label><b>${esc(text(lp.pool_liquidity_raw)||'—')}</b></div><div><label>Permanent lock raw</label><b>${esc(text(lp.permanent_locked_liquidity_raw)||'—')}</b></div><div><label>Permanent locked share</label><b>${esc(pct(lp.permanent_locked_share_pct))}</b></div><div><label>Ownership model</label><b>POSITION NFT</b></div></div>`:`<div class="lp-metrics"><div><label>LP mint</label>${copy(lp.lp_mint)}<b>${esc(number(lp.lp_supply))}</b></div><div><label>Burn share</label><b>${esc(pct(lp.burned_share_pct))}</b></div><div><label>Creator LP share</label><b>${esc(pct(lp.creator_lp_share_pct))}<small>${esc(text(lp.creator_relation)||'—')}</small></b></div><div><label>Dominant LP owner</label>${copy(lp.dominant_lp_owner)}<b>${esc(pct(lp.dominant_lp_share_pct))}<small>${esc(text(lp.dominant_lp_classification)||'—')}</small></b></div><div><label>Locker / unlock</label><b>${esc(short(lp.locker_account))}<small>${esc(text(lp.locked_until)||'—')}</small></b></div></div>`}<div class="lp-movement-head"><h3>${movementTitle}</h3><span class="badge ${movements?'ok':'warn'}">${movements?`${arr(lp.liquidity_movements).length} OBSERVED`:statusLabel(lp.movement_status,lang)}</span></div>${movements?`<div class="lp-table-wrap"><table><thead><tr><th>Type</th><th>Actor wallet</th><th>Token Δ</th><th>Quote Δ</th><th>Signature / slot</th></tr></thead><tbody>${movements}</tbody></table></div>`:movementFallback(lp,lang)}</article>`}
  window.KoscheiLPControlCard={render};
})();
''')

# 7) Durable UI contract verification and CI hook.
Path("koschei/api/scripts/verify-report-trust-consistency.js").write_text(r'''const fs = require('fs');
function need(file, text) {
  const body = fs.readFileSync(file, 'utf8');
  if (!body.includes(text)) throw new Error(`${file} missing ${text}`);
}
need('public/js/public-solana-scan.js', 'Bekleyen kanıt kolları ve izleme pencereleri');
need('public/js/public-solana-scan.js', 'HIZLI ÖN KONTROL');
need('public/js/lp-control-evidence-card.js', 'Havuz hareket geçmişi bu taramada doğrulanamadı');
need('public/index.html', 'signed_verdicts_total');
need('public/index.html', 'KAPSAM SINIRI');
need('public/safe-check.html', 'Holder ve likidite bu sonuçta değerlendirilmedi.');
console.log('report trust consistency contract verified');
''')
replace_once(
    "koschei/api/.github-placeholder" if False else ".github/workflows/api-ci.yml",
    '''          node scripts/verify-customer-investigation-ui.js
          node scripts/verify-dossier-contract.mjs
''',
    '''          node scripts/verify-customer-investigation-ui.js
          node scripts/verify-report-trust-consistency.js
          node scripts/verify-dossier-contract.mjs
''',
)
