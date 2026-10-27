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
  return Boolean(report.final_verdict&&typeof report.final_verdict==='object'&&(Array.isArray(report.modules)||report.investigation_coverage||Object.keys(obj(report.actor_investigation)).length));
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
  if(!('market' in report))return '';
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

// Explorer links are built only from base58 values, so report text can never
// become an arbitrary URL.
const base58=value=>/^[1-9A-HJ-NP-Za-km-z]{32,90}$/.test(String(value||''));
const shortRef=value=>{value=String(value||'');return value.length>14?value.slice(0,6)+'…'+value.slice(-6):value;};
const solscan=(kind,value,text)=>base58(value)?`<a href="https://solscan.io/${kind}/${encodeURIComponent(value)}" target="_blank" rel="noopener noreferrer"><code>${esc(text||shortRef(value))}</code></a>`:`<code>${esc(text||value||'—')}</code>`;
const txLinks=(signatures,limit=3)=>{const list=arr(signatures).filter(base58);if(!list.length)return '';return `<p class="car-note">Transactions: ${list.slice(0,limit).map(sig=>solscan('tx',sig)).join(' ')}${list.length>limit?` +${list.length-limit} more in technical evidence`:''}</p>`;};

// The point of an investigation: who controls this target and what that
// wallet has done so far. Every row comes from verified or observed on-chain
// evidence in the retained report; nothing is inferred about a real person.
const eventLabel={mint_created:'Created token',transfer_out:'Sent',transfer_in:'Received',liquidity_remove_activity:'Removed liquidity',liquidity_add_activity:'Added liquidity',swap:'Swapped'};
const when=value=>{const t=Date.parse(value||'');return Number.isFinite(t)?new Date(t).toISOString().replace('T',' ').slice(0,16)+' UTC':'time unknown';};
function arvisOwnerHistory(report){
  const actor=obj(report.actor_investigation),ext=obj(actor.external_discovery);
  if(!Object.keys(actor).length)return '';
  const relation=obj(actor.current_creator_relation),creation=obj(relation.target);
  const track=obj(obj(actor.dossier).track),trackFacts=obj(track.dossier);
  const wallet=String(actor.wallet||creation.creator_wallet||report.wallet||'');
  const isToken=Boolean(creation.mint);
  const attribution=obj(ext.address_attribution),entities=arr(attribution.entities);
  const outcomesReport=obj(ext.creator_outcome_history),outcomes=arr(outcomesReport.outcomes).slice().sort((a,b)=>String(b.created_on_chain_at||'').localeCompare(String(a.created_on_chain_at||'')));
  const lifecycle=obj(obj(ext.created_mint_portfolio).lifecycle_summary);
  const events=arr(obj(ext.behavior_timeline).events).slice().sort((a,b)=>String(b.observed_at||'').localeCompare(String(a.observed_at||'')));
  const relationships=arr(obj(ext.address_relationships).relationships);
  const patterns=arr(obj(ext.behavior_patterns).matches);
  const observations=arr(obj(ext.behavior_summary).primary_observations);
  const funding=obj(actor.funding_origin),fundingPaths=obj(ext.funding_paths);
  const history=obj(ext.address_history);
  const launch=arr(obj(report.launch_forensics).findings);
  const fates=outcomes.reduce((acc,o)=>{acc[o.fate_status]=(acc[o.fate_status]||0)+1;return acc;},{});

  const header=`<div class="cus-overview car-metrics">
      <div><span>${isToken?'Creator / deployer wallet':'Wallet'}</span><b>${solscan('account',wallet)}</b></div>
      <div><span>Tokens created (indexed)</span><b>${esc(track.created_token_count??'UNKNOWN')}</b></div>
      <div><span>Verified creations</span><b>${esc(trackFacts.verified_creator_token_count??outcomesReport.verified_token_count??'UNKNOWN')}</b></div>
      <div><span>First seen</span><b>${esc(track.first_seen_at?when(track.first_seen_at):'UNKNOWN')}</b></div>
      <div><span>Last seen</span><b>${esc(track.last_seen_at?when(track.last_seen_at):'UNKNOWN')}</b></div>
    </div>
    ${isToken?`<p>This token was created by ${solscan('account',wallet)} on ${esc(when(creation.first_observed_at))}${relation.evidence?.program?` via ${esc(relation.evidence.program)}`:''} — ${esc(String(relation.status||creation.verification_status||'unknown').toUpperCase())} by the creation transaction ${solscan('tx',creation.creation_signature,'creation tx')}.</p>`:''}
    <p>${entities.length?`Verified labels: ${entities.map(e=>esc(e.label||e.name||e.entity||'labelled entity')).join(', ')}`:'No verified identity label exists for this wallet. It is described by its on-chain activity only, not as a real person.'}</p>
    ${observations.length?`<ul>${observations.map(o=>`<li>${esc(o)}</li>`).join('')}</ul>`:''}`;

  const tokens=outcomes.length?`<h5>Tokens this wallet created (${outcomes.length} verified${track.created_token_count>outcomes.length?` of ${esc(track.created_token_count)} indexed`:''})</h5>
    <p>${Object.entries(fates).map(([fate,n])=>`<b>${n}</b> ${esc(label(fate))}`).join(' · ')}${lifecycle.average_inactive_age_days!=null?` · inactive after ${esc(Number(lifecycle.average_inactive_age_days).toFixed(2))} days on average`:''}</p>
    <div class="car-table"><table><thead><tr><th>Token</th><th>Created</th><th>Status now</th><th>Liquidity now</th><th>Proof</th></tr></thead><tbody>${outcomes.map(o=>`<tr><td>${solscan('token',o.mint)}</td><td>${esc(when(o.created_on_chain_at))}</td><td>${esc(label(o.fate_status)||'unknown')}</td><td>${o.current_liquidity_usd==null?'UNKNOWN':esc(usd(o.current_liquidity_usd))}</td><td>${solscan('tx',o.creation_signature,'creation tx')}</td></tr>`).join('')}</tbody></table></div>
    <p class="car-note">"Inactive" is the current market state, not proof of a rug pull. ${arr(outcomesReport.limitations).map(esc).join(' ')}</p>`:
    `<p class="cus-empty">No verified token creations were found for this wallet in the investigated window.</p>`;

  const eventRow=e=>`<li><b>${esc(when(e.observed_at))}</b> · ${esc(eventLabel[e.event_type]||label(e.event_type))}${e.amount_native!=null?` ${esc(e.amount_native)} ${esc(e.asset_type||'')}`:''}${e.token_mint?` ${solscan('token',e.token_mint)}`:''}${e.counterparty?` → ${solscan('account',e.counterparty)}`:''} · ${solscan('tx',e.signature,'tx')}</li>`;
  const timeline=events.length?`<h5>Activity timeline (${events.length} decoded events, newest first)</h5><ol class="car-timeline">${events.slice(0,15).map(eventRow).join('')}</ol>${events.length>15?`<details class="cus-details"><summary>${events.length-15} earlier events</summary><ol class="car-timeline">${events.slice(15).map(eventRow).join('')}</ol></details>`:''}`:'';

  const relationRow=r=>`<li>${solscan('account',r.address)} · ${esc(label(r.relation))} · ${esc(r.outbound_transfers??0)} out / ${esc(r.inbound_transfers??0)} in${arr(r.assets).length?` · ${arr(r.assets).map(esc).join(', ')}`:''}${arr(r.token_mints).length?` · tokens ${arr(r.token_mints).map(m=>solscan('token',m)).join(' ')}`:''} · ${esc(when(r.first_observed_at))} – ${esc(when(r.last_observed_at))}${txLinks(r.evidence_signatures,2)}</li>`;
  const money=relationships.length?`<h5>Wallets it moved funds or tokens with (${relationships.length})</h5><ul>${relationships.map(relationRow).join('')}</ul><p class="car-note">A transfer relationship is not proof of common ownership or wrongdoing.</p>`:'';

  const fundingStatus=String(funding.status||'unknown');
  const fundingFound=!/not_observed|unknown|unavailable/.test(fundingStatus);
  const fundingBlock=`<h5>Where its money came from</h5>${fundingFound?`<p>${esc(label(funding.trail_status||fundingStatus))}${funding.source_wallet?` · source ${solscan('account',funding.source_wallet)}`:''}</p>`:`<p><b>UNKNOWN.</b> ${esc(obj(funding.boundary).reason||'The funding origin was not reached in the investigated history.')} ${funding.signatures_scanned?`${esc(funding.signatures_scanned)} signatures were scanned.`:''}</p>`}${arr(fundingPaths.funding_sources).length?`<ul>${arr(fundingPaths.funding_sources).map(f=>`<li>${solscan('account',f.address||f.source||f.wallet)} ${esc(f.amount_native??'')} ${esc(f.asset_type||'')}</li>`).join('')}</ul>`:''}${arr(funding.limitations).length?`<p class="car-note">${arr(funding.limitations).map(esc).join(' ')}</p>`:''}`;

  const patternBlock=patterns.length?`<h5>Behavior patterns checked (${patterns.length})</h5><ul>${patterns.map(p=>`<li><b>${esc(p.label||p.pattern_id)}</b> · ${esc(p.triggered?'OBSERVED':String(p.status||'not observed').replaceAll('_',' ').toUpperCase())}${p.explanation?` — ${esc(p.explanation)}`:''}</li>`).join('')}</ul>`:'';
  const launchBlock=launch.length?`<h5>Launch-window behavior of this token</h5><ul>${launch.map(f=>`<li>${esc(f)}</li>`).join('')}</ul>`:'';
  const coverage=Object.keys(history).length?`<p class="car-note">History read: ${esc(history.signatures_seen??'?')} signatures${history.history_complete===true?' (complete)':' — partial; older activity may exist outside this window'}. ${arr(history.limitations).map(esc).join(' ')}</p>`:'';

  return `<section class="car-card car-owner"><h4>${isToken?'Who is behind this token and what they have done':'What this wallet has done'}</h4>${header}${tokens}${timeline}${money}${fundingBlock}${patternBlock}${launchBlock}${coverage}</section>`;
}

// A triggered shadow rule is surfaced right under the decision so the
// customer sees it first, while it stays explicit that it did not set the grade.
function arvisShadowAlert(report){
  const hits=arr(report.shadow_rules).filter(rule=>rule&&rule.triggered===true);
  if(!hits.length)return '';
  return `<section class="cus-customer-verdict car-alert" data-tone="review" aria-label="Triggered shadow rules"><span>PATTERN DETECTED · GRADE NOT CHANGED</span>
    ${hits.map(rule=>`<h3>${esc(rule.rule_id)} · ${esc(rule.title)}</h3><p>${esc(rule.summary||'')}</p>`).join('')}
    <p class="car-note">This rule runs in shadow mode: it is shown in full but is not part of the signed grade until it is explicitly promoted. Details and every cited token are listed below.</p></section>`;
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
    const line=r=>`<li>${esc(r.summary||r.title||'')}${txLinks(r.signatures)}</li>`;
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
  const row=x=>`<tr><td>${solscan('token',x.mint)}</td><td>${esc(x.created_on_chain_at||'unknown')}</td><td>${esc(label(x.fate_status)||'unknown')}</td><td>${solscan('tx',x.creation_signature,'creation tx')}</td></tr>`;
  return `<section class="car-card"><h4>Shadow rules · do not change the grade</h4>
    <p class="car-note">Trial rules shown in full for review. They are not part of the signed verdict until explicitly promoted.</p>
    ${rules.map(rule=>{const ev=arr(rule.evidence);return `<details class="cus-details"${rule.triggered?' open':''}><summary>${esc(rule.rule_id)} · ${esc(rule.title)} · ${esc(label(rule.status))}</summary>
      <p>${esc(rule.summary||'')}</p>
      <div class="cus-overview car-metrics">
        <div><span>Verified prior tokens</span><b>${esc(rule.verified_prior_tokens??'?')}</b></div>
        <div><span>Inactive / known fate</span><b>${esc(rule.inactive_or_dead??'?')} / ${esc(rule.known_fate_tokens??'?')}</b></div>
        <div><span>Most launches in 1 hour</span><b>${esc(rule.max_launches_per_hour??'?')}</b></div>
        <div><span>Creator wallet</span><b>${solscan('account',rule.creator_wallet)}</b></div>
      </div>
      ${rule.first_launch_at?`<p class="car-note">Launch window: ${esc(rule.first_launch_at)} – ${esc(rule.last_launch_at||'')}</p>`:''}
      ${ev.length?`<details class="cus-details"><summary>All ${ev.length} cited token creations</summary><div class="car-table"><table><thead><tr><th>Token</th><th>Created</th><th>Status now</th><th>Proof</th></tr></thead><tbody>${ev.map(row).join('')}</tbody></table></div></details>`:''}
      <p class="car-note">${arr(rule.limitations).map(esc).join('<br>')}</p></details>`;}).join('')}
  </section>`;
}

const pathTone={open:'review',closed:'observed',not_observed:'observed',unknown:'limited'};
function arvisThreatPaths(report){
  const threat=obj(report.threat_anticipation);
  if(!Object.keys(threat).length)return '';
  const exit=obj(threat.exit_capacity),rug=obj(threat.rug_pathway_assessment);
  const paths=arr(threat.pathways),scenarios=arr(threat.scenarios),watch=arr(threat.watch_signals),missing=arr(threat.missing_evidence);
  const pct=value=>Number.isFinite(Number(value))&&value!==null?Number(value).toFixed(2)+'%':'UNKNOWN';
  return `<section class="car-card"><h4>How the risk could play out</h4>
    <p class="car-note">Technical pathways, not a rug probability or a claim about intent.</p>
    ${exit.available===true?`<div class="cus-overview car-metrics">
      <div><span>Largest risk-bearing owner</span><b>${esc(pct(exit.owner_percentage))}</b></div>
      <div><span>Owner wallet</span><b>${solscan('account',exit.dominant_owner_wallet)}</b></div>
      <div><span>Position value (ref.)</span><b>${esc(usd(exit.owner_reference_usd_value))}</b></div>
      <div><span>Position / liquidity</span><b>${exit.position_liquidity_multiple==null?'UNKNOWN':esc(exit.position_liquidity_multiple)+'x'}</b></div>
      <div><span>Exit capacity</span><b>${esc(label(exit.capacity)||'unknown').toUpperCase()}</b></div>
    </div>${exit.interpretation?`<p class="car-note">${esc(exit.interpretation)}</p>`:''}`:'<p class="cus-empty">Exit capacity could not be computed; it stays UNKNOWN.</p>'}
    ${paths.length?`<ul class="car-paths">${paths.map(p=>`<li data-tone="${esc(pathTone[p.status]||'limited')}"><b>${esc(p.label||p.id)}</b> <em class="car-tag">${esc(label(p.status).toUpperCase())}</em><br>${esc(p.summary||'')}</li>`).join('')}</ul>`:''}
    ${rug.conclusion?`<p><b>Pathway summary:</b> ${esc(rug.conclusion)}</p>`:''}
    ${missing.length?`<p><b>Evidence still missing for a decision:</b> ${missing.map(esc).join(' · ')}</p>`:''}
    ${scenarios.length?`<details class="cus-details"><summary>Possible next behaviors to watch (${scenarios.length})</summary><ul>${scenarios.map(x=>`<li><b>${esc(x.title||x.id)}</b> — ${esc(x.basis||'')}${arr(x.next_signals).length?`<br><span class="car-note">Next signals: ${arr(x.next_signals).map(esc).join(' · ')}</span>`:''}</li>`).join('')}</ul></details>`:''}
    ${watch.length?`<details class="cus-details"><summary>Early-warning triggers (${watch.length})</summary><ul>${watch.map(x=>`<li><b>${esc(x.title||'')}</b> — ${esc(x.trigger||'')}</li>`).join('')}</ul></details>`:''}
  </section>`;
}

function arvisBehaviorRules(report){
  const signals=arr(obj(report.behavior_signals).signals);
  if(!signals.length)return '';
  const state=s=>s.triggered===true?'TRIGGERED':s.evidence_status==='unverified'?'NOT EVALUATED':'NOT TRIGGERED';
  return `<section class="car-card"><h4>Market and holder behavior rules (${signals.length})</h4>
    <p class="car-note">Every rule is listed, including ones that did not fire or could not be evaluated. "Not evaluated" means evidence was missing, not that the behavior is absent.</p>
    ${signals.map(s=>`<article class="car-rule"><div class="cus-subhead"><span>${esc(s.rule_id)} · ${esc(s.title)}</span><b>${esc(state(s))} · ${esc(String(s.evidence_status||'unknown').toUpperCase())}</b></div><p>${esc(s.summary||'')}</p>${txLinks(s.signatures)}</article>`).join('')}
  </section>`;
}

function arvisModules(report){
  const modules=arr(report.modules);
  if(!modules.length)return '';
  const checks=new Map(arr(obj(report.investigation_coverage).arms).filter(a=>a&&a.subchecks&&a.subchecks.checks).map(a=>[a.module_id,a.subchecks]));
  const checkLabel={performed:'done',performed_partial:'partial (time limit)',unknown:'unknown'};
  const moduleState=m=>m.signed===true?(m.verified===true?'VERIFIED':'COMPLETED'):'EVIDENCE PENDING';
  return `<section class="car-card"><h4>All ARVIS modules (${modules.length})</h4>
    <p class="car-note">Each module with its conclusion, every evidence line it produced and the checks it ran. "Done" means a check ran on verified evidence; it does not mean the result is safe.</p>
    ${modules.map(m=>{const sc=checks.get(m.module_id),required=new Set(arr(sc?.required)),ev=arr(m.evidence);
      return `<details class="cus-details car-module"><summary>${esc(m.module||m.module_id)} · ${esc(moduleState(m))}</summary>
        <p>${esc(m.verdict||'No conclusion was returned.')}</p>
        ${sc?`<div class="cus-reasons">${Object.entries(sc.checks).sort().map(([k,v])=>`<code data-state="${esc(v)}">${required.has(k)?'● ':''}${esc(label(k))}: ${esc(checkLabel[v]||v)}</code>`).join('')}</div>`:''}
        ${ev.length?`<ol class="car-evidence">${ev.map(line=>`<li>${esc(typeof line==='string'?line:JSON.stringify(line))}</li>`).join('')}</ol>`:'<p class="cus-empty">This module produced no evidence lines.</p>'}
      </details>`;}).join('')}
  </section>`;
}

function arvisCoverage(report){
  if(!report.investigation_coverage)return '';
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
    ${arms.length&&!arr(report.modules).length?`<details class="cus-details"><summary>Module checks (${arms.length})</summary><p class="car-note">"done" means the check ran on verified evidence; it does not mean the result is safe.</p>
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
  return `<div class="car-report">${arvisOwnerHistory(report)}${arvisDecisionHeader(report)}${arvisShadowAlert(report)}${arvisMarketSnapshot(report)}${arvisRules(report)}${arvisFindings(report)}${arvisThreatPaths(report)}${arvisBehaviorRules(report)}${arvisShadowRules(report)}${arvisModules(report)}${arvisCoverage(report)}${arvisLimits(report)}
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
