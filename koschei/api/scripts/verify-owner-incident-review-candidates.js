/* Dependency-free owner UI regression: node scripts/verify-owner-incident-review-candidates.js */
'use strict';
const assert=require('node:assert/strict');
const fs=require('node:fs');
const vm=require('node:vm');
const path=require('node:path');
const root=path.resolve(__dirname,'..');
const html=fs.readFileSync(path.join(root,'public/owner-production.html'),'utf8');
const src=fs.readFileSync(path.join(root,'public/js/owner-incident-review-candidates.js'),'utf8');

assert.match(html, /id="page-security"><div id="ownerIncidentReviewMount"[^>]*><\/div><div id="securityContent"/);
assert.match(html, /<script src="\/js\/owner-incident-review-candidates\.js\?v=1"><\/script>/);
assert.match(src, /owner_review_required|owner_review|ownerActive/);
assert.doesNotMatch(src, /innerHTML|outerHTML|eval\(/);

class FakeElement{
  constructor(tag){
    this.tag=tag;
    this.children=[];
    this.listeners=new Map();
    this.className='';
    this.textContent='';
    this.hidden=false;
    this.disabled=false;
    this.open=false;
    this.type='';
    const classes=new Set();
    this.classList={
      contains:v=>classes.has(v),
      toggle:(v,on)=>{if(on===undefined)on=!classes.has(v);if(on)classes.add(v);else classes.delete(v);},
      add:v=>classes.add(v),
      remove:v=>classes.delete(v)
    };
  }
  appendChild(child){this.children.push(child);return child}
  replaceChildren(...children){this.children=children}
  addEventListener(type,fn){const values=this.listeners.get(type)||[];values.push(fn);this.listeners.set(type,values)}
  async emit(type){for(const fn of this.listeners.get(type)||[])await fn({type})}
  async click(){return this.emit('click')}
}
function descend(el){
  return [el,...el.children.flatMap(descend)];
}
const alertID='11111111-1111-4111-8111-111111111111';
const example={
  alert_id:alertID,source:'arvis',event_type:'arvis.verdict.created',
  severity:'high',alert_title:'A sample alert <img src=x>',
  target:'target',evidence_state:'persisted_alert_not_independent_evidence_verification',
  review_state:'owner_review_required',created_at:'2026-10-16T21:00:00Z',last_seen_at:'2026-10-17T00:00:00Z',
  occurrence_count:2,stored_payload_sha256:'a'.repeat(64),
  verdict_lineage:{status:'matched_persisted_signed_verdict_metadata_only',verdict_id:'22222222-2222-4222-8222-222222222222',network:'solana-mainnet',authority:'persisted_signed_verdict_metadata_match_only_not_independent_chain_or_signature_verification'},
  payload_hash_scope:'sha256_of_postgresql_jsonb_text_read_snapshot_not_source_signature',chain_evidence_verified:false,
  suggested_incident:{title:'Review ARVIS sample',severity:'high',network:'',target:'target',summary:'Manual review',alert_refs:[alertID]}
};
async function testBrowserFlow(){
  const page=new FakeElement('section');
  const mount=new FakeElement('div');
  const app=new FakeElement('div');
  const original=new FakeElement('div');
  original.textContent='original-security-renderer';
  app.classList.add('hidden');
  let onChange=()=>{};
  let confirmCalls=0;
  let getCount=0,postCount=0,submitted=null,navClicks=0;
  const nav=new FakeElement('button');
  nav.addEventListener('click',()=>{navClicks++});
  const document={
    createElement:tag=>new FakeElement(tag),
    getElementById:id=>({'page-security':page,'ownerIncidentReviewMount':mount,appView:app,securityContent:original}[id]||null),
    querySelector:q=>q==='#desktopNav [data-nav="security"]'?nav:null
  };
  const win={confirm:()=>{confirmCalls++;return true}};
  class Observer{constructor(fn){onChange=fn}observe(){}}
  const fetch=async(url,options={})=>{
    if(options.method==='POST'){
      postCount++;
      submitted=JSON.parse(options.body);
      return {ok:true,json:async()=>({ok:true})};
    }
    getCount++;
    return {ok:true,json:async()=>({ok:true,schema_version:'koschei.owner-incident-review-candidates.v1',suggestions:[example]})};
  };
  vm.runInNewContext(src,{window:win,document,MutationObserver:Observer,fetch,AbortController,setTimeout,clearTimeout});
  await new Promise(r=>setImmediate(r));
  assert.equal(getCount,0,'must not fetch before owner authentication/security navigation');
  app.classList.remove('hidden');
  page.classList.add('active');
  onChange();
  await new Promise(r=>setImmediate(r));
  assert.equal(getCount,1,'security activation must fetch the owner-only queue once');
  assert.equal(postCount,0,'reading a candidate cannot open an incident');
  assert.equal(original.textContent,'original-security-renderer','canonical security section must remain untouched');
  assert.equal(original.children.length,0,'canonical renderer must not be replaced');
  const all=descend(mount);
  assert.ok(all.some(x=>x.tag==='h3'&&x.textContent===example.alert_title),'untrusted title should be literal text');
  const open=all.find(x=>x.tag==='button'&&x.textContent==='Open incident after review');
  const details=all.find(x=>x.tag==='details');
  assert.ok(open&&details);
  assert.ok(all.some(x=>x.textContent==='a'.repeat(64)),'stored JSONB fingerprint missing in review');
  assert.ok(all.some(x=>String(x.textContent||'').includes('does not authenticate an original chain event')),'provenance scope not disclosed');
  assert.ok(all.some(x=>String(x.textContent||'').includes('Not verified')),'unverified chain evidence boundary missing');
  assert.ok(all.some(x=>String(x.textContent||'').includes('matched_persisted_signed_verdict_metadata_only')),'exact verdict match status not shown');
  assert.ok(all.some(x=>x.textContent==='22222222-2222-4222-8222-222222222222'),'persisted verdict identity missing');
  assert.ok(all.some(x=>String(x.textContent||'').includes('not a validated EvidenceRows set')),'cryptographic proof boundary not disclosed');
  assert.equal(open.disabled,true,'candidate must be inspected first');
  await open.click();
  assert.equal(postCount,0,'disabled review flow must never POST before inspection');
  details.open=true;
  await details.emit('toggle');
  assert.equal(open.disabled,false,'operator can explicitly create only after expanding evidence boundary');
  await open.click();
  await new Promise(r=>setImmediate(r));
  assert.equal(confirmCalls,1,'explicit owner confirmation missing');
  assert.equal(postCount,1,'exactly one incident POST expected');
  assert.equal(submitted.alert_refs[0],alertID,'must preserve genuine alert reference');
  assert.equal(submitted.network,'','must not guess network from alert target');
  assert.equal(navClicks,1,'existing incident table must be refreshed via canonical navigation');
  assert.equal(getCount,2,'new queue must be refreshed');
  assert.equal(original.textContent,'original-security-renderer','existing security UI must remain intact');
  app.classList.add('hidden');
  onChange();
  assert.equal(mount.children.length,0,'owner logout must clear cached candidate data');
  app.classList.remove('hidden');
  onChange();
  await new Promise(r=>setImmediate(r));
  assert.equal(getCount,3,'returning to authenticated Security tab must fetch fresh candidates');
  page.classList.remove('active');
  onChange();
  assert.equal(mount.children.length,0,'leaving security should release stale review content');
}
testBrowserFlow().then(()=>console.log('owner incident review UI regression passed')).catch(error=>{console.error(error);process.exitCode=1});
