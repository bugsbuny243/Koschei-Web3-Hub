(()=>{
'use strict';
const kit=window.OwnerRadarKit;
if(!kit||window.__ownerDecisionPersistenceTruthInstalled)return;
window.__ownerDecisionPersistenceTruthInstalled=true;
const obj=v=>v&&typeof v==='object'&&!Array.isArray(v)?v:{};
const raw=v=>typeof v==='string'?v.trim():'';
const nodeFor=v=>typeof v==='string'?document.getElementById(v):v;
const el=(tag,className,value)=>{
  const n=document.createElement(tag);
  n.className=className;
  if(value!==undefined)n.textContent=value;
  return n;
};
let current={root:null,report:null,payload:null};
function reportFor(payload){
  const envelope=obj(payload);
  return obj(envelope.investigation_report||envelope.report||envelope);
}
function authoritativeAction(decision){
  const explicit=raw(decision.investor_action);
  if(explicit)return explicit.replaceAll('_',' ');
  const names={AVOID:'AVOID TARGET',NOT_CLEARED:'DO NOT TREAT AS SAFE',
    REVIEW_FIRST:'REQUIRE EXPERT REVIEW',CLEARED_WITH_LIMITS:'PROCEED WITH LIMITS'};
  return names[raw(decision.decision).toUpperCase()]||'REVIEW RESULT';
}
function setText(node,value){
  if(node&&node.textContent!==value)node.textContent=value;
}
function harmonize(card,report){
  const decision=obj(report.investor_protection_decision);
  const name=raw(decision.decision).toUpperCase();
  const lens=card.querySelector('.koschei-decision-lens');
  if(!name||!lens)return;
  setText(lens.querySelector('.koschei-decision-lens__copy h3'),'Why '+name.replaceAll('_',' ')+'?');
  setText(lens.querySelector('.koschei-decision-lens__copy p'),
    raw(decision.summary)||'Investor Protection Decision is the authoritative action. Remaining evidence limits still apply.');
  const action=lens.querySelector('.koschei-decision-lens__action');
  setText(action,authoritativeAction(decision));
  const policy=raw(decision.execution_action).toLowerCase()||name.toLowerCase();
  if(action&&action.dataset.policy!==policy)action.dataset.policy=policy;
  if(lens.dataset.protectionDecision!==name.toLowerCase())lens.dataset.protectionDecision=name.toLowerCase();
}
function persistenceText(report){
  const value=raw(report.final_verdict_persistence).toLowerCase();
  switch(value){
    case'persisted':return 'Kalıcı hüküm kaydı: persisted. İmzalı üretim kaydı ve veritabanı bütünlüğü farklı denetimlerdir.';
    case'failed':return 'KRİTİK: İmzalı hüküm üretildi ancak kalıcı verdict kaydı BAŞARISIZ. Bu tarama için saklanan hüküm geçmişi doğrulanmış sayılamaz.';
    case'database_unavailable':return 'KRİTİK: Verdict veritabanına erişilemedi; saklanan hüküm geçmişi doğrulanmış sayılamaz.';
    case'not_applicable':return 'Verdict kayıt durumu: bu taramada uygulanamaz; kalıcı kayıt iddiası yok.';
    default:return 'Verdict kalıcılığı: bağımsız olarak doğrulanmadı; signed işareti veritabanına kaydedildiği anlamına gelmez.';
  }
}
function mount(card,report){
  harmonize(card,report);
  let box=card.querySelector('.owner-verdict-persistence-truth');
  if(!box){
    box=el('section','owner-verdict-persistence-truth');
    box.setAttribute('aria-label','Signed verdict persistence boundary');
    box.appendChild(el('strong','', 'SIGNED ≠ PERSISTED ≠ INDEPENDENTLY VERIFIED'));
    box.appendChild(el('p','owner-verdict-persistence-state'));
    box.appendChild(el('p','owner-verdict-persistence-limit',
      'İmzalı deterministik karar, kaynak kanıtların tamamlandığı ya da Ed25519 imzasının burada bağımsız doğrulandığı iddiası değildir.'));
    const head=card.querySelector('.arvis-premium-head');
    if(head&&typeof head.insertAdjacentElement==='function')head.insertAdjacentElement('afterend',box);
    else card.appendChild(box);
  }
  const text=persistenceText(report);
  setText(box.querySelector('.owner-verdict-persistence-state'),text);
  const failed=['failed','database_unavailable'].includes(raw(report.final_verdict_persistence).toLowerCase());
  if(box.dataset.persistenceRisk!==(failed?'failure':'not_confirmed')){
    box.dataset.persistenceRisk=failed?'failure':'not_confirmed';
  }
}
function apply(){
  const {root,report}=current;
  const app=document.getElementById('appView');
  if(!root||!report||app?.classList?.contains('hidden'))return;
  const cards=root.querySelectorAll?.('[data-arvis-premium-card]')||[];
  for(const card of cards)mount(card,report);
}
function accept(root,payload){
  const element=nodeFor(root);
  if(!element||!payload)return;
  current={root:element,report:reportFor(payload),payload};
  apply();
}
function installStyle(){
  if(document.getElementById('ownerVerdictPersistenceTruthStyles'))return;
  const n=document.createElement('style');
  n.id='ownerVerdictPersistenceTruthStyles';
  n.textContent='.owner-verdict-persistence-truth{margin:12px 0;padding:11px 13px;border:1px solid #ffffff38;border-radius:10px;font-size:12px}.owner-verdict-persistence-truth p{margin:7px 0;line-height:1.5}.owner-verdict-persistence-truth[data-persistence-risk="failure"]{border-color:#e66c65;background:#e66c6513}';
  document.head.appendChild(n);
}
installStyle();
const originalRender=kit.renderUnified;
const originalScan=kit.scan;
const originalDefaultRender=kit.render;
const wrapped={
  ...kit,
  renderUnified(root,payload){
    const result=originalRender?.(root,payload);
    accept(root,payload);
    return result;
  },
  render(root,payload){
    const result=originalDefaultRender?.(root,payload);
    accept(root,payload);
    return result;
  },
  async scan(target,root){
    if(typeof originalScan!=='function')throw new Error('Owner radar scan is unavailable');
    const payload=await originalScan(target,root);
    accept(root,payload);
    return payload;
  }
};
Object.defineProperty(wrapped,'lastScan',{
  enumerable:true,
  configurable:true,
  get(){return current.payload||kit.lastScan||null;}
});
window.OwnerRadarKit=wrapped;
if(typeof MutationObserver==='function'){
  const observer=new MutationObserver(records=>{
    if(!current.root)return;
    for(const record of records){
      if(record.addedNodes?.length){apply();return;}
    }
  });
  observer.observe(document.documentElement,{childList:true,subtree:true});
}
})();
