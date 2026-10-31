package http

import "net/http"

// This page is registered through ownerOnly. It makes read-only calls to the
// existing owner-authenticated API; no actor identity, SLA or containment
// capability is represented as verified.
const ownerSOCTriagePageHTML = `<!doctype html>
<html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1">
<meta name="robots" content="noindex,nofollow">
<title>Koschei SOC — Owner Triage</title>
<style>
:root{color-scheme:dark;font:14px system-ui,sans-serif;background:#081119;color:#e2f5f2}
*{box-sizing:border-box}body{margin:0}main{max-width:1280px;margin:auto;padding:28px 18px 60px}
header{display:flex;align-items:center;justify-content:space-between;gap:16px;flex-wrap:wrap}
h1{font-size:clamp(25px,4vw,38px);margin:8px 0}.muted{color:#9cb6b5;line-height:1.6}
a{color:#77e6da}.panel{border:1px solid #244550;border-radius:16px;background:#0d202b;padding:18px;margin-top:20px}
.controls{display:flex;gap:10px;flex-wrap:wrap;align-items:center}
button,select{background:#143744;border:1px solid #41727e;color:#eefaf6;padding:10px;border-radius:10px;font:inherit}
button{cursor:pointer}button:disabled{opacity:.55;cursor:wait}
.summary{display:flex;gap:12px;flex-wrap:wrap;color:#d2e8e4;margin:16px 0}
.summary span{padding:8px 12px;border:1px solid #2a535c;border-radius:10px}
table{width:100%;border-collapse:collapse;min-width:950px}#signed-candidates{min-width:750px}#signed-candidates code{overflow-wrap:anywhere;word-break:break-word}th,td{padding:12px;border-bottom:1px solid #25404a;text-align:left;vertical-align:top;font-size:12px}
th{color:#a8c4c5;font-size:11px;letter-spacing:.04em}td small{display:block;color:#9aaeb1;margin-top:5px}
.scroll{overflow:auto}.boundary{border-left:3px solid #f5d47d;padding:12px;color:#dccdb4;background:#26251b;border-radius:5px}
#status{min-height:22px}code{font-size:12px;color:#c5e2e1}
</style></head>
<body><main>
<header><div><span class="muted">KOSCHEI WEB3 · SECURITY OPERATIONS</span><h1>Owner SOC triage</h1></div><a href="/owner-production">Owner console</a></header>
<p class="muted">Private operational view · case age, persisted assignments, owner escalation requests and manually recorded owner reviews.</p>
<div class="boundary">Operator case state is not an ARVIS security verdict. L1/L2/L3 labels do not grant access or prove analyst acknowledgement. Escalation requests are not delivered notifications. Manual owner reviews are not analyst responses. Optional UTC review attention is owner-configured and never a contractual SLA. SLA is not configured; no breach claim is made.</div>
<section class="panel">
<div class="controls"><label for="case-status">Case status</label><select id="case-status"><option value="">All statuses</option><option value="open">Open</option><option value="investigating">Investigating</option><option value="contained">Contained (operator state)</option><option value="resolved">Resolved</option><option value="closed">Closed</option></select><button id="refresh" type="button">Refresh owner queue</button></div>
<p id="runtime-readiness" class="muted" role="status" aria-live="polite">SOC database read path: unverified.</p>
<p id="status" class="muted" role="status" aria-live="polite">No data loaded.</p>
<div id="summary" class="summary"></div>
<div class="scroll"><table><thead><tr><th>Incident</th><th>Severity / status</th><th>Age / updated</th><th>Assignment label</th><th>Escalation audit state</th><th>Owner review record</th><th>Review attention (not SLA)</th><th>Evidence refs</th></tr></thead><tbody id="cases"></tbody></table></div>
</section>
<section class="panel" aria-labelledby="signed-decisions-heading">
<h2 id="signed-decisions-heading">Authenticated ARVIS decisions for owner review</h2>
<p class="boundary">Read-only verified signed verdicts are NOT SOC incidents, containment decisions or an automatic severity assessment. Each case must be created or linked explicitly through the existing authenticated owner incident workflow.</p>
<p id="signed-decision-status" role="status" aria-live="polite">Signed receipt candidates: unverified.</p>
<div class="scroll"><table id="signed-candidates"><thead><tr><th>Recorded</th><th>Network / target</th><th>Authenticated final</th><th>Owner case</th><th>Copy-ready signed SOC evidence reference</th></tr></thead><tbody id="signed-decision-rows"></tbody></table></div>
</section><p class="muted">Only the latest 100 cases in this view. Empty, unavailable and incomplete data are never interpreted as a safe chain or a healthy SOC.</p>
</main>
<script>
(function(){
"use strict";
var status=document.getElementById("status");
var readiness=document.getElementById("runtime-readiness");
var body=document.getElementById("cases");
var summary=document.getElementById("summary");
var refresh=document.getElementById("refresh");
var filter=document.getElementById("case-status");
var signedStatus=document.getElementById("signed-decision-status");
var signedRows=document.getElementById("signed-decision-rows");
function label(v){return typeof v==="string" && v.trim()?v:"Unknown";}
function text(tag,value){var el=document.createElement(tag);el.textContent=value;return el;}
function cell(row,value,sub){
 var td=text("td",value);if(sub){td.appendChild(text("small",sub));}row.appendChild(td);
}
function duration(v){return Number.isFinite(v)?Math.max(0,Math.floor(v/3600))+"h":"Unknown";}
function render(data){
 body.replaceChildren();summary.replaceChildren();
 if(!data||!Array.isArray(data.items)){status.textContent="Response has no validated queue data.";return;}
 var counts=data.counts_by_status||{};
 summary.appendChild(text("span","Cases in window: "+String(data.count)));
 summary.appendChild(text("span","Open: "+String(counts.open||0)));
 summary.appendChild(text("span","Investigating: "+String(counts.investigating||0)));
 summary.appendChild(text("span","SLA: NOT CONFIGURED"));
 var policy=data.review_policy;
 summary.appendChild(text("span",policy&&policy.policy_id?
   "Review policy: "+label(policy.policy_id)+" · elapsed UTC":"Review attention: NOT CONFIGURED"));
 if(!data.items.length){status.textContent="No persisted incidents returned for this filter. This is not a security verdict.";return;}
 for(var item of data.items){
   var inc=item.incident||{},row=document.createElement("tr");
   var link=document.createElement("a");
   link.textContent=label(inc.incident_ref);
   var id=typeof inc.id==="string"?inc.id:"";
   if(/^[0-9a-fA-F-]{36}$/.test(id)){link.href="/api/owner/incidents/"+encodeURIComponent(id);}
   else {link.removeAttribute("href");}
   var td=document.createElement("td");td.appendChild(link);td.appendChild(text("small",label(inc.title)));
   if(/^[0-9a-fA-F-]{36}$/.test(id)){
     var handoff=document.createElement("a");
     handoff.textContent="Open handoff JSON";
     handoff.href="/api/owner/incidents/"+encodeURIComponent(id)+"?handoff=1";
     td.appendChild(text("small","Audit snapshot · not evidence verification"));
     td.appendChild(handoff);
   }
   row.appendChild(td);
   cell(row,label(inc.severity),label(inc.status));
   cell(row,duration(item.age_seconds),duration(item.since_last_update_seconds)+" since last update");
   var a=item.analyst_assignment;
   cell(row,a&&a.status==="assigned"?label(a.tier)+" · "+label(a.analyst_id):"No verified analyst",a?"Owner label: "+label(a.status):"No assignment event");
   var e=item.escalation;
   cell(row,e?label(e.state):"No escalation event",e&&e.target_tier?"Target: "+label(e.target_tier):"");
   var review=item.owner_review;
   cell(row,review&&review.evidence_state==="owner_manual_review_recorded"?"Owner review recorded":"No owner review record",
       review?"First recorded after "+duration(review.seconds_from_case_creation)+" · "+String(review.record_count)+" record(s)":"No review audit event");
   var attention=item.review_attention;
   var attentionLabel="No configured review window";
   var attentionDetail="";
   if(attention){
     attentionLabel=label(attention.state).replaceAll("_"," ");
     if(Number.isFinite(attention.over_target_seconds)){
       attentionDetail=duration(attention.over_target_seconds)+" past review target (NOT SLA breach)";
     }else if(attention.review_at){
       attentionDetail="Review target: "+label(attention.review_at);
     }else{
       attentionDetail="Not measured; no review deadline inferred";
     }
   }
   cell(row,attentionLabel,attentionDetail);
   var refs=Array.isArray(inc.evidence_refs)?inc.evidence_refs.length:0;
   cell(row,String(refs),"References only; verification is separate");
   body.appendChild(row);
 }
 status.textContent="Read-only snapshot · "+label(data.as_of)+" · "+data.items.length+" returned cases.";
}
async function loadCandidates(){
 signedRows.replaceChildren();
 signedStatus.textContent="Reading independently authenticated decision candidates...";
 try{
   var response=await fetch("/api/owner/soc/verdict-candidates",{method:"GET",credentials:"same-origin",cache:"no-store",headers:{"Accept":"application/json"}});
   if(!response.ok){throw new Error("Trusted candidate proof unavailable (HTTP "+response.status+")");}
   var data=await response.json();
   if(data.ok!==true||!Array.isArray(data.items)||data.schema_version!=="koschei.owner-soc-signed-candidates.v1"){throw new Error("Candidate schema was not validated");}
   for(var candidate of data.items){
     if(!candidate||typeof candidate.evidence_ref!=="string"||!candidate.evidence_ref.startsWith("unified-verdict-ledger:")){throw new Error("Candidate evidence contract invalid");}
   }
   signedStatus.textContent=data.items.length+" independently verified signed finals in the bounded recent sample. A decision is not a SOC case.";
   if(!data.items.length){
     signedRows.appendChild(text("tr","No signed token decisions in the verified sample. This does not prove that the whole ledger is empty."));
     return;
   }
   for(var item of data.items){
     var tr=document.createElement("tr");
     cell(tr,label(item.recorded_at));
     cell(tr,label(item.network),label(item.target));
     cell(tr,label(item.grade),label(item.verdict));
     cell(tr,item.linked_to_soc===true?"Linked to Owner SOC":"Owner review required","No autonomous action");
     var td=document.createElement("td"),ref=text("code",item.evidence_ref),copy=text("button","Copy reference");
     copy.type="button";copy.addEventListener("click",async function(value){return async function(){
       try{await navigator.clipboard.writeText(value);signedStatus.textContent="Signed evidence reference copied. Owner action is still required.";}
       catch(_){signedStatus.textContent="Clipboard not available. Select the displayed reference manually.";}
     };}(item.evidence_ref));
     td.appendChild(ref);td.appendChild(document.createElement("br"));td.appendChild(copy);tr.appendChild(td);signedRows.appendChild(tr);
   }
 }catch(e){
   signedRows.replaceChildren();
   signedStatus.textContent="Signed ARVIS candidates UNVERIFIED / UNAVAILABLE. No pending cases or safe verdicts may be inferred.";
 }
}
async function loadReadiness(){
 readiness.textContent="SOC storage: verifying read path...";
 try{
   var response=await fetch("/api/owner/soc/readiness",{method:"GET",credentials:"same-origin",cache:"no-store",headers:{"Accept":"application/json"}});
   if(!response.ok){throw new Error("read probes unavailable");}
   var data=await response.json();
   if(data.ok!==true||!data.readiness){throw new Error("unverified read response");}
   var snapshot=data.readiness;
   var operations=snapshot.operations;
   var receipts=snapshot.signed_verdict_receipts;
   if(!receipts||typeof receipts.state!=="string"){throw new Error("Missing receipt proof status");}
   readiness.textContent="Operations primary storage: "+(operations?label(operations.state):"UNVERIFIED")+" · Legacy SOC storage: "+label(snapshot.state)+
     " · sampled cases: "+(Number.isInteger(snapshot.sampled_cases)?String(snapshot.sampled_cases):"unknown")+
     " · audited actions observed: "+(snapshot.any_audited_actions===true?"yes":"no")+
     " · cases linked to audit rows: "+(Number.isInteger(snapshot.linked_audit_case_samples)?String(snapshot.linked_audit_case_samples):"unknown")+
     " · Signed ARVIS receipts: "+label(receipts.state)+" ("+String(receipts.verified_receipts||0)+"/"+String(receipts.sampled_receipts||0)+")"+
     " · NOT end-to-end accepted";
 }catch(err){
   readiness.textContent="SOC storage: UNAVAILABLE / NOT VERIFIED. No healthy state can be inferred.";
 }
}
async function load(){
 refresh.disabled=true;status.textContent="Loading owner-only queue...";
 body.replaceChildren();summary.replaceChildren();
 try{
   var query="/api/owner/incidents?triage=1&limit=100";
   if(filter.value){query+="&status="+encodeURIComponent(filter.value);}
   var response=await fetch(query,{method:"GET",credentials:"same-origin",cache:"no-store",headers:{"Accept":"application/json"}});
   if(!response.ok){throw new Error("Queue unavailable or session unauthorized (HTTP "+response.status+").");}
   var data=await response.json();
   if(data.ok!==true){throw new Error("Owner queue did not confirm a valid response.");}
   render(data);
 }catch(err){status.textContent=String(err&&err.message||"Queue unavailable")+" No empty/healthy state is inferred.";}
 finally{refresh.disabled=false;}
}
refresh.addEventListener("click",function(){loadReadiness();loadCandidates();load();});filter.addEventListener("change",load);loadReadiness();loadCandidates();load();
})();
</script></body></html>`

func ownerSOCTriagePage(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Robots-Tag", "noindex, nofollow")
	_, _ = w.Write([]byte(ownerSOCTriagePageHTML))
}
