(()=>{
'use strict';
if(window.__koscheiInvestorProtectionV1)return;
window.__koscheiInvestorProtectionV1=true;

const text=value=>String(value??'').replace(/\s+/g,' ').trim();
const obj=value=>value&&typeof value==='object'&&!Array.isArray(value)?value:{};
const arr=value=>Array.isArray(value)?value:[];
const el=(tag,className,content)=>{const node=document.createElement(tag);if(className)node.className=className;if(content!==undefined)node.textContent=content;return node;};

function currentReport(){
  const payload=obj(window.OwnerRadarKit?.lastScan);
  return obj(payload.investigation_report||payload.report||payload);
}

// A remembered scan cannot supply a decision for a different premium card.
// Owner Assistant and the dedicated Owner Radar do not share a lifecycle.
function reportForCard(card){
  const report=currentReport();
  const visibleTarget=text(card.querySelector('.arvis-target')?.textContent);
  const reportTarget=text(report.target||obj(report.final_verdict).target);
  if(visibleTarget&&(!reportTarget||visibleTarget!==reportTarget))return {};
  return report;
}

function protectionActionLabel(decision){
  const explicit=text(decision.investor_action).replaceAll('_',' ');
  if(explicit)return explicit;
  switch(text(decision.decision).toUpperCase()){
    case'AVOID':return'AVOID TARGET';
    case'NOT_CLEARED':return'DO NOT TREAT AS SAFE';
    case'REVIEW_FIRST':return'REQUIRE EXPERT REVIEW';
    case'CLEARED_WITH_LIMITS':return'PROCEED WITH LIMITS';
    default:return'REVIEW RESULT';
  }
}

// Owner assistant premium cards can render outside OwnerRadarKit.scan.
// When lastScan is stale, the backend decision already projected into the
// SAME card must take precedence over the legacy local policy chip.
function decisionFromCard(card){
  const section=card.querySelector('.koschei-investor-protection');
  if(!section)return null;
  const decision=text(section.dataset?.decision).toUpperCase();
  if(!['AVOID','NOT_CLEARED','REVIEW_FIRST','CLEARED_WITH_LIMITS'].includes(decision))return null;
  const summary=text(section.querySelector('.koschei-investor-protection__top p')?.textContent);
  const action=text(section.querySelector('.koschei-investor-protection__action')?.textContent);
  let execution=text(section.dataset?.executionAction);
  for(const fact of section.querySelectorAll('.koschei-investor-protection__facts > div')){
    if(text(fact.querySelector('span')?.textContent).toLowerCase()==='execution'){
      execution=text(fact.querySelector('b')?.textContent)||execution;
      break;
    }
  }
  return {decision,summary,investor_action:action,execution_action:execution};
}

function harmonizeDecisionLens(card,decision){
  const lens=card.querySelector('.koschei-decision-lens');
  if(!lens)return;
  const authoritative=decisionFromCard(card)||obj(decision);
  const decisionName=text(authoritative.decision).toUpperCase();
  if(!decisionName)return;

  const title=lens.querySelector('.koschei-decision-lens__copy h3');
  const copy=lens.querySelector('.koschei-decision-lens__copy p');
  const action=lens.querySelector('.koschei-decision-lens__action');
  if(title)title.textContent=`Why ${decisionName.replaceAll('_',' ')}?`;
  if(copy)copy.textContent=text(authoritative.summary)||'The investor protection contract is authoritative for the customer action.';
  if(action){
    action.textContent=protectionActionLabel(authoritative);
    action.dataset.policy=text(authoritative.execution_action).toLowerCase()||
      (decisionName==='AVOID'?'block':text(authoritative.decision).toLowerCase());
  }

  // Once the backend protection contract exists, the old local policy chip is
  // context only. It must never render a softer action than the backend truth.
  lens.dataset.protectionDecision=decisionName.toLowerCase();
  const note=lens.querySelector('.koschei-ux-note');
  if(note)note.textContent='Investor Protection Decision is authoritative. Legacy policy labels are retained only as rule-engine context.';
}

function renderProtection(card){
  const report=reportForCard(card);
  const decision=obj(report.investor_protection_decision);
  if(!text(decision.decision))return;

  card.querySelector('.koschei-investor-protection')?.remove();
  const section=el('section','koschei-investor-protection');
  section.dataset.decision=text(decision.decision).toLowerCase();
  section.dataset.executionAction=text(decision.execution_action).toLowerCase();
  section.setAttribute('aria-label','Investor protection decision');

  const top=el('div','koschei-investor-protection__top');
  const copy=el('div','');
  copy.append(
    el('span','koschei-investor-protection__kicker','INVESTOR PROTECTION DECISION'),
    el('h2','',text(decision.decision).replaceAll('_',' ')),
    el('p','',text(decision.summary)||'Koschei has not issued a safety clearance for this target.')
  );
  const action=el('strong','koschei-investor-protection__action',protectionActionLabel(decision));
  top.append(copy,action);section.appendChild(top);

  const facts=el('div','koschei-investor-protection__facts');
  const fact=(label,value)=>{const item=el('div','');item.append(el('span','',label),el('b','',text(value)||'—'));facts.appendChild(item);};
  fact('Grade',decision.grade);
  fact('Verdict',decision.verdict);
  fact('Execution',decision.execution_action);
  fact('Cleared',decision.cleared===true?'YES':'NO');
  section.appendChild(facts);

  const basis=arr(decision.basis);
  if(basis.length){
    const list=el('div','koschei-investor-protection__basis');
    list.appendChild(el('h3','','Why this decision'));
    for(const item of basis.slice(0,6)){
      const row=el('div','koschei-investor-protection__basis-row');
      row.append(
        el('b','',text(item.rule_id)||text(item.code)||'Evidence rule'),
        el('span','',text(item.evidence_status).toUpperCase()),
        el('p','',text(item.summary)||'Evidence-backed risk rule triggered.')
      );
      list.appendChild(row);
    }
    section.appendChild(list);
  }

  const gaps=arr(decision.critical_gaps);
  if(gaps.length){
    const gap=el('div','koschei-investor-protection__gaps');
    gap.appendChild(el('h3','',`Not collected in this scan (${gaps.length})`));
    for(const item of gaps.slice(0,5))gap.appendChild(el('p','',`${text(item.capability)||'Capability'}: ${text(item.reason)||text(item.status)||'unavailable'}`));
    section.appendChild(gap);
  }

  const head=card.querySelector('.arvis-premium-head');
  if(head)head.insertAdjacentElement('afterend',section);
  else card.prepend(section);
  harmonizeDecisionLens(card,decision);
}

function enhance(root=document){
  root.querySelectorAll?.('[data-arvis-premium-card]').forEach(renderProtection);
}

const observer=new MutationObserver(records=>{
  for(const record of records){
    for(const node of record.addedNodes){
      if(node.nodeType!==1)continue;
      if(node.matches?.('[data-arvis-premium-card]')||node.querySelector?.('[data-arvis-premium-card]'))enhance(node.parentElement||node);
      // The UX layer may insert the Decision Lens *after* this renderer has
      // already inserted Investor Protection. Re-apply the authoritative
      // backend decision as soon as that late lens enters the DOM.
      const newLens=node.matches?.('.koschei-decision-lens')?node:node.querySelector?.('.koschei-decision-lens');
      const newProtection=node.matches?.('.koschei-investor-protection')?node:node.querySelector?.('.koschei-investor-protection');
      const card=(newLens||newProtection)?.closest?.('[data-arvis-premium-card]');
      if(card)harmonizeDecisionLens(card,obj(reportForCard(card).investor_protection_decision));
    }
  }
});
observer.observe(document.documentElement,{childList:true,subtree:true});
if(document.readyState==='loading')document.addEventListener('DOMContentLoaded',()=>enhance());else enhance();
window.addEventListener('koschei:investigation-rendered',()=>enhance());
})();
