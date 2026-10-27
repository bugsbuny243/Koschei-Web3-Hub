(()=>{
'use strict';
if(window.__koscheiUniversalAddressScanV1)return;
window.__koscheiUniversalAddressScanV1=true;

const esc=value=>String(value??'').replace(/[&<>"']/g,ch=>({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[ch]));
const yesNo=value=>value===true?'YES':value===false?'NO':'UNKNOWN';
const short=value=>String(value??'')||'—';
const errorCopy={401:'Sign in to continue this analysis.',402:'Professional access is required for this request.',403:'Your account cannot access this request.',429:'Too many requests. Wait a moment, then try again.',503:'This evidence source is currently unavailable. Try again later.'};

function authorityRows(authority){
  if(!authority)return'<div class="cus-empty">No authority snapshot was attached for this network.</div>';
  const rows=[
    ['Contract code',authority.contract_code_state],
    ['Code hash',authority.contract_code_sha256],
    ['EIP-7702',authority.delegation_state],
    ['Delegate target',authority.delegation_target],
    ['ERC-1967',authority.proxy_state],
    ['Implementation',authority.proxy_implementation],
    ['Admin',authority.proxy_admin],
    ['Beacon',authority.proxy_beacon],
    ['Conflicting slots',authority.conflicting_slots===true?'YES':authority.conflicting_slots===false?'NO':'—']
  ];
  return rows.filter(([,value])=>value!==undefined&&value!==null&&value!=='').map(([key,value])=>`<div class="cus-row"><span>${esc(key)}</span><b title="${esc(value)}">${esc(short(value))}</b></div>`).join('');
}

function renderTrust(trust={}){
  const axes=[['Claimed',trust.claimed],['Observed',trust.observed],['Authorized',trust.authorized],['Available',trust.available],['Verified',trust.verified],['Finalized',trust.finalized]];
  return `<div class="cus-trust">${axes.map(([name,value])=>`<div data-on="${value===true?'1':'0'}"><span>${esc(name)}</span><b>${yesNo(value)}</b></div>`).join('')}</div>`;
}

function customerSummary(result){
  if(!result)return'Evidence unavailable';
  if(result.transaction_evidence){
    const state=String(result.transaction_evidence.state_change||result.transaction_evidence.attributes?.execution_state||'unknown').replaceAll('_',' ');
    return 'Transaction '+state;
  }
  if(result.evm_authority){
    const a=result.evm_authority;
    if(a.delegation_state==='delegation_observed')return'EIP-7702 delegation observed';
    if(a.proxy_state==='erc1967_authority_observed')return'Upgradeable authority observed';
    if(a.contract_code_state==='contract_code_observed')return'Contract code observed';
  }
  if(result.trust?.observed===true)return'On-chain state observed';
  return String(result.status||'Evidence returned').replaceAll('_',' ');
}

function verdictSummary(classification,entries){
  const live=entries.filter(x=>x.ok&&x.data?.result);
  if(!live.length){
    return{tone:'limited',label:'LIMITED EVIDENCE',title:'No live evidence source returned a result.',copy:'Koschei could not complete a live observation for this address. Missing evidence is not treated as a safety signal.'};
  }
  const results=live.map(x=>x.data.result);
  const authorities=results.map(x=>x.evm_authority).filter(Boolean);
  const delegated=authorities.some(a=>a.delegation_state==='delegation_observed');
  const upgradeable=authorities.some(a=>a.proxy_state==='erc1967_authority_observed');
  const conflicting=authorities.some(a=>a.conflicting_slots===true);
  if(conflicting){
    return{tone:'review',label:'REVIEW AUTHORITY',title:'Conflicting proxy authority slots were observed.',copy:'The address has a control surface that deserves review. This observation does not prove malicious intent.'};
  }
  if(delegated){
    return{tone:'review',label:'REVIEW AUTHORITY',title:'EIP-7702 delegation was observed.',copy:'The address delegates execution authority. Review the delegate target and attached evidence before relying on the address.'};
  }
  if(upgradeable){
    return{tone:'review',label:'REVIEW AUTHORITY',title:'Upgradeable authority was observed.',copy:'ERC-1967 authority is present on at least one observed network. Upgradeability is a control surface, not an automatic malicious verdict.'};
  }
  const observed=results.some(x=>x.trust?.observed===true);
  if(observed){
    return{tone:'observed',label:'OBSERVED',title:'Live on-chain state was observed.',copy:classification.family==='evm'?'No supported authority signal was attached to the returned observations. This is not proof that the address is immutable or safe.':'Koschei returned live read-only evidence. Review the evidence details before making a security decision.'};
  }
  return{tone:'limited',label:'LIMITED EVIDENCE',title:'Evidence returned without a completed observation.',copy:'Koschei did not promote incomplete evidence into a safety claim.'};
}

function assetRows(evidence){
  if(!evidence)return '';
  const a=evidence.attributes||{};
  const rows=[
    ['Interface',a.interface_state],
    ['Name',a.name],
    ['Symbol',a.symbol],
    ['Decimals',a.decimals],
    ['Total supply',a.total_supply],
    ['Contract balance',a.contract_balance],
    ['Compliance claimed',a.standards_compliance_claimed===true?'YES':a.standards_compliance_claimed===false?'NO':undefined]
  ];
  return '<div class="cus-authority"><div class="cus-subhead"><span>Asset surface</span><b>Observed only</b></div>'+
    rows.filter(([,value])=>value!==undefined&&value!==null&&value!=='').map(([key,value])=>'<div class="cus-row"><span>'+esc(key)+'</span><b title="'+esc(value)+'">'+esc(short(value))+'</b></div>').join('')+'</div>';
}

function transactionRows(evidence){
  if(!evidence)return '';
  const a=evidence.attributes||{};
  const rows=[
    ['Execution',evidence.state_change||a.execution_state],
    ['Block',evidence.block_or_slot||a.block_number],
    ['From',evidence.address||a.from],
    ['To',evidence.contract||a.to],
    ['Input selector',a.input_selector],
    ['Selector hint',a.input_selector_hint],
    ['Receipt status',a.receipt_status],
    ['Logs',a.log_count],
    ['Standard events',a.standard_event_count],
    ['Transfer events',a.transfer_event_count],
    ['Approval events',a.approval_event_count],
    ['BTC inputs',a.input_count],
    ['BTC outputs',a.output_count],
    ['Total input sats',a.total_input_sats],
    ['Total output sats',a.total_output_sats],
    ['Derived fee sats',a.derived_fee_sats],
    ['Fee balance checked',a.fee_balance_checked===true?'YES':a.fee_balance_checked===false?'NO':undefined],
    ['Fee consistent',a.fee_consistent===true?'YES':a.fee_consistent===false?'NO':undefined],
    ['Input script types',a.input_script_types?JSON.stringify(a.input_script_types):undefined],
    ['Output script types',a.output_script_types?JSON.stringify(a.output_script_types):undefined]
  ];
  return '<div class="cus-authority"><div class="cus-subhead"><span>Transaction evidence</span><b>Observed only</b></div>'+
    rows.filter(([,value])=>value!==undefined&&value!==null&&value!=='').map(([key,value])=>'<div class="cus-row"><span>'+esc(key)+'</span><b title="'+esc(value)+'">'+esc(short(value))+'</b></div>').join('')+'</div>';
}

// Customer decision availability is derived from the server's per-network
// investigation plan, never from an owner privilege or an address's syntax.
// This read-only /api/scan response does NOT contain a verified signed verdict.
// Preserve every returned native-adapter field for the customer, including
// provenance, metrics and new attributes not yet assigned a compact UI label.
function fullEvidencePanel(label,evidence){
  if(!evidence)return '';
  const details=JSON.stringify(evidence,null,2);
  return `<details class="cus-details cus-deep-evidence" open><summary>${esc(label)} — complete collector evidence</summary><pre class="cus-raw">${esc(details)}</pre></details>`;
}

function customerDecisionAvailability(result){
  const plan=result?.investigation_plan;
  if(result?.target?.requires_network===true)return{label:'NETWORK REQUIRED',copy:'Select the target network before interpreting evidence or requesting a security decision.'};
  if(!plan)return{label:'DECISION NOT AVAILABLE',copy:'A supported network investigation plan has not been established. No risk grade or safety conclusion has been issued.'};
  if(plan.executable!==true)return{label:'COLLECTOR NOT CONNECTED',copy:'This network is registered but its customer evidence collector is not executable. A risk grade is unavailable.'};
  switch(String(plan.verdict_authority||'')){
    case 'existing_deterministic_rules_only':
      return{label:'SIGNED VERDICT NOT PROVIDED',copy:'Deterministic rules are declared for this network, but this customer evidence request alone does not provide a verified signed final decision.'};
    case 'evidence_only_no_grade_authority':
      return{label:'EVIDENCE ONLY — NO RISK GRADE',copy:'Network observations can be inspected. This adapter is not authorized to assign a signed risk grade; observed activity is not proof of safety.'};
    default:
      return{label:'NO CONNECTED VERDICT AUTHORITY',copy:'No customer risk grade is available for this network. Collected evidence must not be treated as a safety decision.'};
  }
}

function resultCard(entry){
  const result=entry.data?.result;
  if(!entry.ok||!result){
    const message=errorCopy[entry.http]||entry.data?.error||'The evidence request could not be completed. Try again.';
    return `<article class="cus-network-card is-unavailable"><div class="cus-card-head"><div><span>${esc(entry.label)}</span><b>Evidence unavailable</b></div><em>${esc(String(entry.http||'OFFLINE'))}</em></div><p>${esc(message)}</p></article>`;
  }
  const authority=result.evm_authority;
  const reasons=Array.isArray(result.reasons)?result.reasons:[];
  const decision=customerDecisionAvailability(result);
  return `<article class="cus-network-card">
    <div class="cus-card-head"><div><span>${esc(entry.label)}</span><b>${esc(customerSummary(result))}</b></div><em>${esc(String(result.evidence_status||'unknown').toUpperCase())}</em></div>
    <div class="cus-target">${esc(result.target?.raw||'')}</div>
    <details class="cus-details" open><summary>All collected evidence and provenance</summary>
      ${renderTrust(result.trust||{})}
      ${fullEvidencePanel('Native network observation',result.network_evidence)}
      ${authority?`<div class="cus-authority"><div class="cus-subhead"><span>Authority surface</span><b>Observed evidence</b></div>${authorityRows(authority)}</div>`:''}
      ${assetRows(result.asset_evidence)}
      ${transactionRows(result.transaction_evidence)}
      ${fullEvidencePanel('Authority investigation',authority)}
      ${fullEvidencePanel('Asset investigation',result.asset_evidence)}
      ${fullEvidencePanel('Transaction investigation',result.transaction_evidence)}
      <div class="cus-reasons"><div class="cus-subhead"><span>Reason codes</span><b>${reasons.length}</b></div>${reasons.length?reasons.map(reason=>`<code>${esc(reason)}</code>`).join(''):'<span class="cus-empty">No reason code attached.</span>'}</div>
      <div class="cus-reasons"><div class="cus-subhead">Evidence references</div>${(result.evidence_refs||[]).map(ref=>`<code>${esc(ref)}</code>`).join('')||'<span class="cus-empty">No reference was attached.</span>'}</div>
      <details class="cus-details"><summary>Complete response</summary><pre class="cus-raw">${esc(JSON.stringify(entry.data,null,2))}</pre></details>
    </details>
    <div class="cus-decision-policy" role="status" aria-label="Customer decision availability"><b>${esc(decision.label)}</b><p>${esc(decision.copy)}</p></div>
  </article>`;
}

function summaryBlock(classification,entries){
  const live=entries.filter(x=>x.ok&&x.data?.result);
  const unavailable=entries.length-live.length;
  const authorityCount=live.filter(x=>Boolean(x.data?.result?.evm_authority)).length;
  const observed=live.filter(x=>x.data?.result?.trust?.observed===true).length;
  const verdict=verdictSummary(classification,entries);
  return `<section class="cus-customer-verdict" data-tone="${esc(verdict.tone)}">
      <span>${esc(verdict.label)}</span><h3>${esc(verdict.title)}</h3><p>${esc(verdict.copy)}</p>
    </section>
    <section class="cus-overview">
      <div><span>Detected</span><b>${esc(classification.label)}</b></div>
      <div><span>Evidence returned</span><b>${live.length}</b></div>
      <div><span>Observed networks</span><b>${observed}</b></div>
      <div><span>Unavailable</span><b>${unavailable}</b></div>
      ${classification.family==='evm'?`<div><span>Authority snapshots</span><b>${authorityCount}</b></div>`:''}
    </section>`;
}

// Readable customer view of a retained Solana ARVIS report. Every value comes
// from the server report; absent fields stay UNKNOWN and never read as safe.
const arr=value=>Array.isArray(value)?value:[];
const obj=value=>value&&typeof value==='object'&&!Array.isArray(value)?value:{};
const usd=value=>Number.isFinite(Number(value))&&value!==null&&value!==''?'$'+Number(value).toLocaleString('en-US',{maximumFractionDigits:Number(value)<1?8:0}):'UNKNOWN';
const label=value=>String(value??'').replaceAll('_',' ').trim();

function readableArvisReport(report){
  report=obj(report);
  return Boolean(report.final_verdict&&typeof report.final_verdict==='object'&&(Array.isArray(report.modules)||report.investigation_coverage));
}

function arvisDecisionHeader(report){
  const verdict=obj(report.final_verdict),decision=obj(report.investor_protection_decision);
  const grade=String(verdict.grade||'').trim();
  const graded=verdict.signed===true&&grade!==''&&grade!=='-';
  const tone=!graded?'limited':decision.decision==='CLEARED'?'observed':'review';
  const title=graded?`Signed ARVIS grade ${grade}`:'No signed grade was issued';
  const copy=decision.summary||'A grade is not a safety certificate. Missing evidence is never treated as safe.';
  const meta=[
    decision.decision&&`Decision: ${label(decision.decision)}`,
    verdict.verdict&&`Verdict path: ${label(verdict.verdict)}`,
    verdict.ruleset_version&&`Ruleset ${verdict.ruleset_version}`,
    verdict.signed===true?`Signed ${verdict.generated_at||verdict.created_at||''}`.trim():'Unsigned'
  ].filter(Boolean);
  return `<section class="cus-customer-verdict car-decision" data-tone="${esc(tone)}" aria-label="ARVIS decision">
    <span>${esc(graded?'SIGNED VERDICT':'NOT GRADED')}</span>
    <div class="car-grade-row"><b class="car-grade">${esc(graded?grade:'—')}</b><h3>${esc(title)}</h3></div>
    <p>${esc(copy)}</p>
    <p class="car-meta">${meta.map(esc).join(' · ')}</p>
    ${verdict.signature?`<p class="car-meta">Signature <code>${esc(String(verdict.signature).slice(0,24))}…</code>${verdict.key_id?` · key ${esc(verdict.key_id)}`:''}</p>`:''}
  </section>`;
}

function arvisMarketSnapshot(report){
  const market=obj(report.market);
  if(market.available!==true)return `<section class="car-card"><h4>Market snapshot</h4><p class="cus-empty">No verified market snapshot was returned. Liquidity and price are UNKNOWN.</p></section>`;
  const name=[market.name,market.symbol&&`(${market.symbol})`].filter(Boolean).join(' ')||'Name not observed';
  return `<section class="car-card"><h4>${esc(name)}</h4>
    <div class="cus-overview car-metrics">
      <div><span>Liquidity</span><b>${esc(usd(market.liquidity_usd))}</b></div>
      <div><span>Market cap</span><b>${esc(usd(market.market_cap_usd))}</b></div>
      <div><span>24h volume</span><b>${esc(usd(market.volume_24h_usd))}</b></div>
      <div><span>Price</span><b>${esc(usd(market.price_usd))}</b></div>
      <div><span>24h buys / sells</span><b>${esc(market.buys_24h??'?')} / ${esc(market.sells_24h??'?')}</b></div>
    </div>
    <p class="car-note">Point-in-time snapshot from ${esc(market.provider||'an external market source')} at ${esc(market.observed_at||'an unknown time')}; not a guaranteed exit price.</p></section>`;
}

function arvisRuleGroups(rules){
  const groups=new Map();
  for(const rule of arr(rules)){
    const id=String(rule?.rule_id||'rule');
    if(!groups.has(id))groups.set(id,{id,title:rule.title||id,status:rule.evidence_status,rows:[]});
    groups.get(id).rows.push(rule);
  }
  return [...groups.values()];
}

function arvisRules(report){
  const verdict=obj(report.final_verdict),groups=arvisRuleGroups(verdict.triggered_rules),watch=arr(verdict.watch_flags);
  const group=g=>{
    const first=g.rows.slice(0,3),rest=g.rows.slice(3);
    const line=r=>`<li>${esc(r.summary||r.title||'')}</li>`;
    return `<article class="car-rule"><div class="cus-subhead"><span>${esc(g.id)} · ${esc(g.title)}</span><b>${esc(String(g.status||'unknown').toUpperCase())} · ${g.rows.length}</b></div><ul>${first.map(line).join('')}</ul>${rest.length?`<details class="cus-details"><summary>${rest.length} more</summary><ul>${rest.map(line).join('')}</ul></details>`:''}</article>`;
  };
  return `<section class="car-card"><h4>Rules that set the grade</h4>
    ${groups.length?groups.map(group).join(''):'<p class="cus-empty">No grade-changing rule was triggered. This does not mean the token is safe.</p>'}
    ${watch.length?`<details class="cus-details"><summary>Watch flags · do not change the grade (${watch.length})</summary>${watch.map(w=>`<p class="car-note"><b>${esc(w.rule_id||'')} ${esc(w.title||'')}</b> — ${esc(w.summary||'')}</p>`).join('')}</details>`:''}
  </section>`;
}

function arvisFindings(report){
  const signals=arr(obj(report.actor_coordination_intelligence).signals);
  const exposure=obj(report.threat_anticipation).primary_exposure;
  if(!signals.length&&!exposure)return '';
  return `<section class="car-card"><h4>Key findings</h4>
    ${exposure?`<p><b>Primary exposure:</b> ${esc(exposure)}</p>`:''}
    ${signals.map(s=>`<p>${esc(s.summary||label(s.kind))} <em class="car-tag">${esc(String(s.evidence_status||'unknown').toUpperCase())}</em></p>`).join('')}
    <p class="car-note">Relationships describe on-chain technical links only. They are not identity, intent or wrongdoing claims.</p></section>`;
}

function arvisShadowRules(report){
  const rules=arr(report.shadow_rules);
  if(!rules.length)return '';
  return `<section class="car-card"><h4>Shadow rules · do not change the grade</h4>
    <p class="car-note">Trial rules shown for review. They are not part of the signed verdict until explicitly promoted.</p>
    ${rules.map(rule=>`<details class="cus-details"${rule.triggered?' open':''}><summary>${esc(rule.rule_id)} · ${esc(rule.title)} · ${esc(label(rule.status))}</summary>
      <p>${esc(rule.summary||'')}</p>
      ${arr(rule.evidence).length?`<p class="car-note">${arr(rule.evidence).length} cited token creations with signatures are included in the technical evidence.</p>`:''}
      <p class="car-note">${arr(rule.limitations).map(esc).join('<br>')}</p></details>`).join('')}
  </section>`;
}

function arvisCoverage(report){
  const coverage=obj(report.investigation_coverage),arms=arr(coverage.arms).filter(a=>a&&a.subchecks&&a.subchecks.checks);
  const checkLabel={performed:'done',performed_partial:'partial (time limit)',unknown:'unknown'};
  const gaps=arr(coverage.critical_gaps);
  return `<section class="car-card"><h4>Investigation coverage</h4>
    <div class="cus-overview car-metrics">
      <div><span>Status</span><b>${esc(label(coverage.status)||'unknown')}</b></div>
      <div><span>Modules completed</span><b>${esc(coverage.completed??'?')} / ${esc(coverage.capability_total??'?')}</b></div>
      <div><span>Produced evidence</span><b>${esc(coverage.evidence_producing??'?')}</b></div>
      <div><span>Evidence pending</span><b>${esc(coverage.evidence_pending??'?')}</b></div>
      <div><span>Source unavailable</span><b>${esc(coverage.source_unavailable??'?')}</b></div>
    </div>
    ${coverage.summary?`<p class="car-note">${esc(coverage.summary)}</p>`:''}
    ${gaps.length?`<p><b>Critical gaps:</b> ${gaps.map(esc).join(' · ')}</p>`:''}
    ${arms.length?`<details class="cus-details"><summary>Module checks (${arms.length})</summary><p class="car-note">"done" means the check ran on verified evidence; it does not mean the result is safe.</p>
      ${arms.map(a=>{const sc=a.subchecks,required=new Set(arr(sc.required));return `<div class="car-module"><div class="cus-subhead"><span>${esc(a.module||a.module_id)}</span><b>${esc(label(sc.required_checks_status)||'unknown')}</b></div><div class="cus-reasons">${Object.entries(sc.checks).sort().map(([k,v])=>`<code data-state="${esc(v)}">${required.has(k)?'● ':''}${esc(label(k))}: ${esc(checkLabel[v]||v)}</code>`).join('')}</div></div>`;}).join('')}</details>`:''}
  </section>`;
}

function arvisLimits(report){
  const decision=obj(report.investor_protection_decision),seen=new Set(),limits=[];
  for(const limit of [...arr(decision.critical_gaps),...arr(decision.evidence_limits)]){
    const text=typeof limit==='string'?limit:[limit?.capability,limit?.reason].filter(Boolean).join(': ');
    if(text&&!seen.has(text)){seen.add(text);limits.push(text);}
  }
  if(!limits.length)return '';
  const first=limits.slice(0,5),rest=limits.slice(5);
  return `<section class="car-card"><h4>What is unknown or limited</h4><ul>${first.map(x=>`<li>${esc(x)}</li>`).join('')}</ul>${rest.length?`<details class="cus-details"><summary>${rest.length} more limitations</summary><ul>${rest.map(x=>`<li>${esc(x)}</li>`).join('')}</ul></details>`:''}</section>`;
}

function readableArvisReportHTML(report){
  return `<div class="car-report">${arvisDecisionHeader(report)}${arvisMarketSnapshot(report)}${arvisRules(report)}${arvisFindings(report)}${arvisShadowRules(report)}${arvisCoverage(report)}${arvisLimits(report)}
    <details class="cus-details car-technical" data-arvis-technical><summary>Technical evidence (JSON)</summary><p class="car-note">The complete retained report with every collector row and provenance. It is rendered when opened.</p><button type="button" data-arvis-download>Download JSON</button><pre class="cus-raw" data-arvis-raw></pre></details>
    <p class="car-note">Koschei never converts missing evidence into a safe result. A grade is not a safety certificate or financial advice.</p></div>`;
}

// The full report can be megabytes; render or download it only on request.
function bindTechnicalEvidence(root,report){
  const details=root?.querySelector?.('[data-arvis-technical]');
  if(!details)return;
  details.addEventListener('toggle',()=>{
    const raw=details.querySelector('[data-arvis-raw]');
    if(details.open&&raw&&!raw.textContent)raw.textContent=JSON.stringify(report,null,2);
  });
  details.querySelector('[data-arvis-download]')?.addEventListener('click',()=>{
    const blob=new Blob([JSON.stringify(report,null,2)],{type:'application/json'});
    const link=document.createElement('a');
    link.href=URL.createObjectURL(blob);
    link.download='koschei-arvis-'+String(report.target||'report').slice(0,48)+'.json';
    link.click();
    setTimeout(()=>URL.revokeObjectURL(link.href),1000);
  });
}

function install(){
  const section=document.getElementById('customerUniversalScan'),routing=window.KoscheiScanEntry;
  if(!section||!routing)return;
  const $=id=>document.getElementById(id);
  const shared=Boolean($('radarMode'));
  const form=shared?$('scanForm'):$('customerUniversalScanForm'),input=shared?$('target'):$('customerUniversalTarget'),network=shared?$('scanNetwork'):$('customerUniversalNetwork');
  const submit=shared?$('submit'):$('customerUniversalSubmit'),cancel=$('customerUniversalCancel'),status=$('customerUniversalStatus');
  const active=()=>!shared||['address','lookup'].includes(window.KoscheiRadar?.mode);
  const wrap=$('customerUniversalResultsWrap'),overview=$('customerUniversalOverview'),results=$('customerUniversalResults');
  const recovery=$('customerUniversalRecovery'),advanced=$('advancedTools');
  let pending=null,generation=0,deepAbort=null,deepPollTimer=null;

  function stop(){
    generation++;
    pending?.abort();pending=null;
    deepAbort?.abort();deepAbort=null;
    if(deepPollTimer!==null){clearTimeout(deepPollTimer);deepPollTimer=null;}
    form.removeAttribute('aria-busy');if(active()){submit.disabled=false;submit.textContent=shared?'Investigate target':'Analyze target';}cancel.hidden=true;
  }
  function invalidate(){
    stop();wrap.hidden=true;recovery.hidden=true;
    status.textContent='Ready to analyze the current target and network.';
    input.removeAttribute('aria-invalid');
  }
  input.addEventListener('input',()=>{if(active())invalidate();});network.addEventListener('change',()=>{if(active())invalidate();});
  if(shared)document.addEventListener('koschei:radar-mode-changed',()=>{stop();wrap.hidden=true;recovery.hidden=true;status.textContent='Ready. Enter a target for this investigation.';});
  cancel.addEventListener('click',()=>{if(!active())return;stop();wrap.hidden=true;status.textContent='Analysis canceled. You can start another request.';});
  window.addEventListener('pagehide',stop);

  // The canonical customer job endpoint is already account-scoped and durable.
  // Never route this through owner APIs or claim non-Solana collectors exist.
  // The account's newest retained job for this target, or null. Any failure
  // (signed out, no Professional access, network) means "nothing to reuse".
  async function latestRetainedJob(request,signal){
    try{
      const query=new URLSearchParams({target:request.target,network:request.network});
      const response=await window.KoscheiAuth.apiCall('/api/v1/radar/jobs/?'+query,{credentials:'same-origin',cache:'no-store',headers:{Accept:'application/json'},signal});
      if(!response?.ok)return null;
      const item=(await response.json())?.history?.[0];
      if(!item||item.target!==request.target||item.network!==request.network||!/^[0-9a-f-]{36}$/i.test(String(item.id||'')))return null;
      return item;
    }catch{return null;}
  }
  const RETAINED_REUSE_MS=30*60*1000;

  // A Solana address investigation starts with an explicit scan (auto); a
  // shared link only reattaches to retained work (resume). Either way a
  // running job is followed, and a recent finished report is shown, before
  // any new output is spent. Other networks still wait for a click.
  function offerCanonicalDeepJob(request,{auto=false,resume=false}={}){
    const button=document.createElement('button');
    button.type='button';
    const nativeOnly=request.family!=='solana';
    button.textContent=request.kind==='transaction'?'Run and save transaction evidence':nativeOnly?'Run and save native evidence':'Run durable ARVIS investigation';
    const start=async({fresh=true,reuseOnly=false}={})=>{
      const current=generation;
      if(deepAbort)return;
      const controller=new AbortController();deepAbort=controller;
      button.disabled=true;
      if(!reuseOnly)status.textContent='Creating a durable, account-scoped investigation…';
      try{
        if(!window.KoscheiAuth?.apiCall){
          if(!reuseOnly)status.textContent='Sign in to create an account-scoped investigation.';
          button.disabled=false;deepAbort=null;return;
        }
        await window.KoscheiAuth.init();
        if(!window.KoscheiAuth.isLoggedIn()){
          if(!reuseOnly)status.textContent='Sign in to start this retained investigation.';
          button.disabled=false;deepAbort=null;return;
        }
        if(!nativeOnly&&!fresh){
          const latest=await latestRetainedJob(request,controller.signal);
          if(current!==generation)return;
          const age=Date.now()-Date.parse(latest?.updated_at||'');
          if(latest&&['queued','running'].includes(latest.status)){
            follow(String(latest.id),{joined:true});return;
          }
          if(latest&&latest.status==='completed'&&latest.result_available===true&&(reuseOnly||age<RETAINED_REUSE_MS)){
            follow(String(latest.id),{retainedAt:latest.updated_at});return;
          }
          if(reuseOnly){button.disabled=false;deepAbort=null;return;}
        }
        const response=await window.KoscheiAuth.apiCall('/api/v1/radar/jobs',{
          method:'POST',credentials:'same-origin',cache:'no-store',
          headers:{'Content-Type':'application/json','Accept':'application/json'},
          body:JSON.stringify({address:request.target,network:request.network,max_depth:nativeOnly?0:3}),
          signal:controller.signal
        });
        if(!response)throw new Error('Authenticated job transport unavailable.');
        const job=await response.json();
        if(current!==generation)return;
        if(!response.ok){
          status.textContent=response.status===401?'Sign in to run a deep investigation.':
            [402,403].includes(response.status)?'An active Professional account is required for a deep investigation.':
            'The deep investigation could not be started. No report was created.';
          button.disabled=false;deepAbort=null;return;
        }
        const id=String(job?.job_id||'');
        if(!/^[0-9a-f-]{36}$/i.test(id)||job.poll_url!=='/api/v1/radar/jobs/'+id||job.canonical_engine!==true){
          throw new Error('The background job confirmation could not be authenticated.');
        }
        follow(id,{joined:job?.joined_existing_job===true});
      }catch(error){
        if(current!==generation||controller.signal.aborted)return;
        status.textContent='Deep investigation request failed: '+(error.message||'unavailable');
        button.disabled=false;deepAbort=null;
      }

      // Follows one account-scoped job to its terminal state. joined: an
      // in-flight job was reused; retainedAt: a finished report is reopened.
      async function follow(id,{joined=false,retainedAt=''}={}){
        const pollURL='/api/v1/radar/jobs/'+id;
        if(retainedAt)button.textContent='Run a fresh investigation';
        button.disabled=false;
        // The API status resource requires a bearer session; opening it as a
        // bare browser link would fail authentication. Use the signed-in
        // account history page to retrieve the result with its auth helper.
        const historyLink=document.createElement('a');
        historyLink.href='/reports?job='+encodeURIComponent(id);
        historyLink.textContent='View retained result in account history';
        recovery.replaceChildren(button,historyLink);
        recovery.hidden=false;
        // The address pre-check stays available but folds away behind the
        // investigation, which is what the customer asked for.
        const initialHTML=nativeOnly?results.innerHTML:
          `<details class="cus-details car-precheck"><summary>Pre-check · live account observation</summary>${results.innerHTML}</details>`;
        if(!nativeOnly)overview.innerHTML='<section class="cus-customer-verdict" data-tone="observed"><span>INVESTIGATING</span><h3>ARVIS is collecting evidence for this target.</h3><p>The full investigation usually takes one to four minutes. No grade is shown until the signed verdict is ready.</p></section>';
        let polls=0;
        // Leave the INVESTIGATING headline only while the job is in flight.
        const settleOverview=(label,title,copy)=>{if(current!==generation||nativeOnly)return;
          overview.innerHTML=label?`<section class="cus-customer-verdict" data-tone="limited"><span>${esc(label)}</span><h3>${esc(title)}</h3><p>${esc(copy)}</p></section>`:'';};
        const show=(message,report)=>{if(current!==generation)return;
          const readable=!nativeOnly&&report&&readableArvisReport(report);
          if(readable)overview.innerHTML='';
          results.innerHTML=initialHTML+
            `<section class="cus-deep-evidence" aria-label="Durable customer investigation"><h3>${nativeOnly?'Native evidence investigation':'ARVIS investigation'}</h3><p>${esc(message)}</p>`+
            `<p>Account-scoped job: <code>${esc(id)}</code></p>`+
            (readable?readableArvisReportHTML(report):report?fullEvidencePanel('Complete retained ARVIS report',report):'')+
            '</section>';
          if(readable)bindTechnicalEvidence(results,report);
        };
        const retainedNote=retainedAt?`Showing your retained report from ${retainedAt}; no output was used. "Run a fresh investigation" starts a new one (1 output).`:'';
        show(retainedAt?'Loading your retained report…':'Queued. The investigation continues on the server and remains in your account history.');
        status.textContent=retainedNote||(joined?'This target is already being investigated for your account; the running job was reused and no additional output was used.':'ARVIS investigation started; one Professional output is used for this report.');
        const poll=async()=>{
          if(current!==generation||controller.signal.aborted)return;
          try{
            const response=await window.KoscheiAuth.apiCall(pollURL,{credentials:'same-origin',cache:'no-store',headers:{Accept:'application/json'},signal:controller.signal});
            if(!response?.ok)throw new Error('Investigation status is temporarily unavailable.');
            const snapshot=await response.json();
            if(current!==generation)return;
            if(snapshot.id!==id||snapshot.job_type!=='canonical_investigation'||
               snapshot.target!==request.target||snapshot.network!==request.network)
              throw new Error('The background report does not match this customer target/network.');
            if(snapshot.status==='completed'){
              if(!snapshot.result||typeof snapshot.result!=='object'||Array.isArray(snapshot.result))
                throw new Error('Job completed without readable evidence; no verdict can be inferred.');
              show(nativeOnly?'Native collection finished. All observed evidence is retained; historical collectors and a signed grade are NOT claimed.':'Completed. All retained findings and provenance are shown below.',snapshot.result);
              status.textContent=retainedNote||(nativeOnly?'Native evidence job completed; the result is retained in account history.':'ARVIS job completed; the report is retained in account history.');
              settleOverview();
              deepAbort=null;return;
            }
            if(snapshot.status==='failed'){
              show('The background investigation failed. Existing scan evidence remains visible; no complete report is claimed.');
              status.textContent='Deep investigation failed. Review account history for retry status.';
              settleOverview('INVESTIGATION FAILED','No ARVIS report was produced.','The pre-check below is not a security decision. Missing evidence is not treated as safe.');
              deepAbort=null;return;
            }
            if(snapshot.status!=='queued'&&snapshot.status!=='running')
              throw new Error('The investigation returned an unknown state; no complete report is claimed.');
            const progress=Number(snapshot.progress);
            const percentage=Number.isInteger(progress)&&progress>=0&&progress<=100?` (${progress}%)`:'';
            show('Background investigation '+snapshot.status+percentage+'. Evidence is retained by the job service.');
            if(++polls>=60){
              show('Still processing. Automatic checks have stopped; the durable job continues in your account history.');
              settleOverview('STILL PROCESSING','The investigation continues on the server.','Open your account history later to see the retained report.');
              deepAbort=null;return;
            }
            deepPollTimer=setTimeout(poll,5000);
          }catch(error){
            if(current!==generation||controller.signal.aborted)return;
            show('Report polling interrupted: '+(error.message||'unknown')+'. The durable job may still be running.');
            status.textContent='Open your investigation history to inspect the retained job.';
            settleOverview('STATUS UNAVAILABLE','This page stopped following the investigation.','The job may still finish; its report is retained in your account history.');
            deepAbort=null;
          }
        };
        await poll();
      }
    };
    button.addEventListener('click',()=>start({fresh:true}));
    const chat=document.createElement('a');
    chat.href='/arvis-chat?'+new URLSearchParams({target:request.target,network:request.network});
    chat.textContent='Explore evidence with ARVIS';
    recovery.replaceChildren(button,chat);recovery.hidden=false;
    if(auto)start({fresh:false});
    else if(resume&&!nativeOnly)start({fresh:false,reuseOnly:true});
  }

  // Only an explicit form submission may start a paid investigation; opening a
  // shared or bookmarked result link runs the free pre-check only.
  async function runScan(explicit=false){
    if(!active())return;
    stop();wrap.hidden=true;recovery.hidden=true;
    const request=routing.resolve(input.value,network.value);
    if(request.error){
      status.textContent=request.error;input.setAttribute('aria-invalid','true');
      (request.needsNetwork?network:input).focus();return;
    }
    input.removeAttribute('aria-invalid');network.value=request.network;
    const current=++generation,controller=new AbortController();pending=controller;
    const timer=setTimeout(()=>controller.abort(),40000);
    submit.disabled=true;submit.textContent='Analyzing…';cancel.hidden=false;form.setAttribute('aria-busy','true');
    status.textContent=`Collecting read-only ${request.kind==='transaction'?'transaction':'address'} evidence on ${request.label}…`;
    history.replaceState({},'',routing.url(request.target,request.network).url);
    try{
      const response=await fetch('/api/scan',{method:'POST',headers:{'Content-Type':'application/json','Accept':'application/json'},body:JSON.stringify({target:request.target,network:request.network}),cache:'no-store',credentials:'same-origin',signal:controller.signal});
      const data=await response.json();
      if(current!==generation)return;
      if(response.ok&&!routing.matchesResult(data,request))throw new Error('The returned evidence does not match this target and network. The result was withheld.');
      const entry={network:request.network,label:request.label,http:response.status,ok:response.ok,data};
      overview.innerHTML=summaryBlock({family:request.family,label:request.label+' '+(request.kind==='transaction'?'transaction':'address')},[entry]);
      results.innerHTML=resultCard(entry);wrap.hidden=false;
      if(shared)$('empty').hidden=true;
      status.textContent=response.ok?'Analysis complete. Review the findings, limits and technical evidence below.':(errorCopy[response.status]||'Analysis could not complete. Review the source error below and try again.');
      if([401,402,403].includes(response.status)){
        const link=document.createElement('a');
        link.href=response.status===401?'/login?next='+encodeURIComponent(routing.url(request.target,request.network).url):'/account';
        link.textContent=response.status===401?'Sign in to continue':'Review account access';
        recovery.replaceChildren(link);recovery.hidden=false;
      }
      const durableAddress=request.kind==='address'&&['solana','evm','bitcoin'].includes(request.family);
      const durableTransaction=request.kind==='transaction'&&['evm','bitcoin'].includes(request.family);
      if(response.ok&&(durableAddress||durableTransaction)){
        const solanaAddress=durableAddress&&request.family==='solana';
        offerCanonicalDeepJob(request,{auto:explicit&&solanaAddress,resume:!explicit&&solanaAddress});
      }
    }catch(error){
      if(current!==generation)return;
      status.textContent=controller.signal.aborted?'The evidence service did not respond within 40 seconds. Some network sources may be unavailable; try again.':(error instanceof SyntaxError?'The service returned an unreadable response. Try again.':error.message||'Connection failed. Try again.');
      wrap.hidden=true;
    }finally{
      clearTimeout(timer);
      if(current===generation){pending=null;submit.disabled=false;submit.textContent=shared?'Investigate target':'Analyze target';cancel.hidden=true;form.removeAttribute('aria-busy');}
    }
  }
  form.addEventListener('submit',event=>{if(!active())return;event.preventDefault();runScan(true);});
  const params=new URLSearchParams(location.search),addressView=routing.isAddressView();
  if(advanced){
    advanced.open=!addressView;
    document.querySelectorAll('[data-scan-mode]').forEach(button=>button.addEventListener('click',()=>{stop();advanced.open=true;}));
  }
  const initial=params.get('target');
  if(addressView&&initial){
    input.value=initial;network.value=params.get('network')||'';
    if(params.get('network')&&!routing.networks.some(item=>item[0]===params.get('network'))){status.textContent='The link contains an unsupported network. Select a network to continue.';return;}
    runScan();
  }
}

document.readyState==='loading'?document.addEventListener('DOMContentLoaded',install,{once:true}):install();
})();
