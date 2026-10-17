// Additive owner-only incident review desk. Deliberately outside the canonical
// securityContent renderer: its refreshes must not delete or duplicate this UI.
(()=>{
  'use strict';
  if(window.__koscheiOwnerIncidentReviewInstalled)return;
  const section=document.getElementById('page-security');
  const mount=document.getElementById('ownerIncidentReviewMount');
  if(!section||!mount)return;
  window.__koscheiOwnerIncidentReviewInstalled=true;

  const ENDPOINT='/api/owner/incidents';
  const SCHEMA='koschei.owner-incident-review-candidates.v1';
  let active=false;
  let loading=false;
  let requestVersion=0;

  function node(tag,className,text){
    const el=document.createElement(tag);
    if(className)el.className=className;
    if(text!==undefined)el.textContent=String(text);
    return el;
  }
  function add(parent,...children){
    for(const child of children){if(child)parent.appendChild(child)}
    return parent;
  }
  function ownerActive(){
    const app=document.getElementById('appView');
    return section.classList.contains('active')&&app&&!app.classList.contains('hidden');
  }
  function statusCard(title,message,canRetry){
    const card=node('article','card');
    const heading=node('h2','',title);
    const description=node('p','muted',message);
    add(card,heading,description);
    if(canRetry){
      const retry=node('button','btn','Retry review candidates');
      retry.type='button';
      retry.addEventListener('click',load);
      card.appendChild(retry);
    }
    mount.replaceChildren(card);
  }
  async function request(url,options){
    const controller=new AbortController();
    const timer=setTimeout(()=>controller.abort(),20000);
    try{
      const response=await fetch(url,{
        credentials:'same-origin',cache:'no-store',signal:controller.signal,
        headers:{'Accept':'application/json',...(options?.body?{'Content-Type':'application/json'}:{})},
        ...(options||{})
      });
      if(!response.ok)throw new Error('HTTP '+response.status);
      const json=await response.json();
      if(json?.ok!==true)throw new Error('Owner API rejected the request');
      return json;
    }finally{clearTimeout(timer)}
  }
  function labelled(parent,label,value){
    const row=node('div','muted small');
    add(row,node('strong','',label+': '),node('span','',value||'Not supplied'));
    parent.appendChild(row);
  }
  function candidateCard(item){
    const card=node('article','card');
    const title=String(item.alert_title||'Persisted alert').slice(0,180);
    const severity=String(item.severity||'unknown').toLowerCase();
    const refs=item.suggested_incident;
    add(card,node('div','eyebrow','ALERT REVIEW · '+severity.toUpperCase()),node('h3','',title));
    labelled(card,'Alert ID',item.alert_id);
    labelled(card,'Source',item.source);
    labelled(card,'Event',item.event_type);
    labelled(card,'Target',item.target);
    labelled(card,'Observed',item.last_seen_at);
    const detail=node('details','');
    const summary=node('summary','','Inspect evidence boundary and proposed case');
    const explanation=node('p','muted','This is a persisted alert, not independent proof of an exploit or malicious intent. Inspect the original evidence before opening an incident.');
    add(detail,summary,explanation);
    labelled(detail,'Evidence',item.evidence_state);
    labelled(detail,'Review state',item.review_state);
    labelled(detail,'First recorded',item.created_at);
    labelled(detail,'Last seen',item.last_seen_at);
    labelled(detail,'Occurrences',item.occurrence_count==null?'Not supplied':String(item.occurrence_count));
    const digest=String(item.stored_payload_sha256||'');
    const validDigest=/^[a-f0-9]{64}$/.test(digest);
    labelled(detail,'Stored JSONB SHA-256',validDigest?digest:'Unavailable');
    labelled(detail,'Fingerprint scope',item.payload_hash_scope||'Unavailable');
    add(detail,node('p','muted small','This fingerprint covers the stored PostgreSQL JSONB text snapshot only. It does not authenticate an original chain event, an alert producer or immutable history.'));
    labelled(detail,'Independent chain proof','Not verified by this projection');
    const lineage=item.verdict_lineage&&typeof item.verdict_lineage==='object'?item.verdict_lineage:{};
    labelled(detail,'Persisted verdict link',lineage.status||'Unavailable');
    labelled(detail,'Verdict record ID',lineage.verdict_id||'Unresolved');
    labelled(detail,'Recorded verdict network',lineage.network||'Unresolved');
    labelled(detail,'Link assurance',lineage.authority||'Exact persisted match not independently verified');
    add(detail,node('p','muted small','A matching database verdict is not a validated EvidenceRows set, independently checked Ed25519 signature, or blockchain inclusion proof.'));
    labelled(detail,'Proposed network',refs&&refs.network?refs.network:'Unresolved; no inference from target');
    labelled(detail,'Proposed summary',refs&&refs.summary);
    const open=node('button','btn','Open incident after review');
    open.type='button';
    open.disabled=true;
    const alertID=String(item.alert_id||'');
    const validID=/^[a-f0-9]{8}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{12}$/i.test(alertID);
    // Only a valid persisted alert reference and explicit expansion permits a
    // subsequent user-confirmed POST. The server independently checks the ref.
    detail.addEventListener('toggle',()=>{
      open.disabled=!detail.open||!validID||!refs||loading;
    });
    open.addEventListener('click',async()=>{
      if(open.disabled||!detail.open||!validID||!refs||loading)return;
      if(!window.confirm('Create an operator incident linked to alert '+alertID+'? This requires evidence review; the alert alone does not prove compromise.'))return;
      open.disabled=true;
      const body={
        title:String(refs.title||'Review persisted security alert').slice(0,180),
        severity:['info','low','medium','high','critical'].includes(String(refs.severity))?refs.severity:'medium',
        network:'',
        target:String(refs.target||'').slice(0,512),
        summary:String(refs.summary||'Manual alert review').slice(0,4000),
        alert_refs:[alertID],
        evidence_refs:[],
        dossier_refs:[]
      };
      try{
        await request(ENDPOINT,{method:'POST',body:JSON.stringify(body)});
        statusCard('Incident created','The existing owner incident flow accepted the verified alert reference. Refreshing the review queue.',false);
        // The canonical owner security renderer remains the source of truth
        // for the incident table and existing operator actions.
        const securityNav=document.querySelector('#desktopNav [data-nav="security"]');
        if(securityNav)securityNav.click();
        await load();
      }catch(error){
        statusCard('Incident creation not confirmed','The server did not confirm the operation: '+error.message+'. Check the existing incident list before retrying.',true);
      }
    });
    add(detail,open);
    card.appendChild(detail);
    return card;
  }
  function render(data){
    const items=Array.isArray(data.suggestions)?data.suggestions.slice(0,50):[];
    const root=node('div','');
    const header=node('article','card');
    add(header,node('div','eyebrow','READ-ONLY · OPERATOR TRIAGE'),node('h2','','Review candidates'),node('p','muted','Recent high/critical persisted alerts not linked to an active incident. A severity label is not an independently verified security finding. No incident is opened by viewing this list.'));
    const reload=node('button','btn','Refresh candidates');
    reload.type='button';reload.addEventListener('click',load);
    header.appendChild(reload);
    root.appendChild(header);
    if(!items.length){
      root.appendChild(node('div','empty','No eligible persisted alerts in the bounded window. This does not mean the network or assets are safe.'));
    }else{
      const grid=node('div','grid compact-grid');
      for(const item of items){
        if(!item||typeof item!=='object')continue;
        const wrap=node('div','span-12');
        wrap.appendChild(candidateCard(item));
        grid.appendChild(wrap);
      }
      root.appendChild(grid);
    }
    mount.replaceChildren(root);
  }
  async function load(){
    if(!ownerActive()||loading)return;
    loading=true;
    const version=++requestVersion;
    statusCard('Incident review candidates','Reading recent persisted alerts; existing incident management remains unchanged.',false);
    try{
      const result=await request(ENDPOINT+'?suggestions=1&limit=50');
      if(version!==requestVersion||!ownerActive())return;
      if(result.schema_version!==SCHEMA)throw new Error('Incident review contract is unavailable');
      render(result);
    }catch(error){
      if(version===requestVersion&&ownerActive()){
        statusCard('Review candidates unavailable','No alert safety assumption can be made. '+error.message,true);
      }
    }finally{
      loading=false;
      // A tab switch while a request was in flight may have skipped the
      // re-entry fetch. Resume only for the current authenticated tab.
      if(version!==requestVersion&&ownerActive())void load();
    }
  }
  function onPageChange(){
    const next=!!ownerActive();
    if(next&&!active){active=true;load()}
    else if(!next&&active){active=false;requestVersion++;mount.replaceChildren()}
  }
  // Observe only existing navigation state. Never wrap/replace core functions,
  // intercept existing handlers, or observe the mutable securityContent subtree.
  new MutationObserver(onPageChange).observe(section,{attributes:true,attributeFilter:['class']});
  const app=document.getElementById('appView');
  if(app)new MutationObserver(onPageChange).observe(app,{attributes:true,attributeFilter:['class']});
  onPageChange();
})();
