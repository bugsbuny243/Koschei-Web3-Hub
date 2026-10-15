const {test}=require('node:test');
const assert=require('node:assert/strict');
const fs=require('node:fs');
const path=require('node:path');
const vm=require('node:vm');

test('late owner decision lens honors signed AVOID/BLOCK instead of earlier REVIEW chip',()=>{
 const source=fs.readFileSync(path.join(__dirname,'owner-investor-protection.js'),'utf8');
 const title={textContent:'Why REVIEW?'};
 const copy={textContent:'No deterministic blocking rule fired.'};
 const action={textContent:'REVIEW RESULT',dataset:{policy:'review'}};
 const lens={
  nodeType:1,dataset:{},
  matches:selector=>selector==='.koschei-decision-lens',
  querySelector:selector=>({
   '.koschei-decision-lens__copy h3':title,
   '.koschei-decision-lens__copy p':copy,
   '.koschei-decision-lens__action':action
  })[selector]||null,
  closest:selector=>selector==='[data-arvis-premium-card]'?card:null
 };
 const card={querySelector:selector=>selector==='.koschei-decision-lens'?lens:null};
 let observe;
 class MutationObserver {
  constructor(callback){observe=callback;}
  observe(){}
 }
 const window={
  OwnerRadarKit:{lastScan:{investigation_report:{
   investor_protection_decision:{
    decision:'AVOID',execution_action:'BLOCK',
    investor_action:'AVOID TARGET',
    summary:'Verified owner concentration triggered the deterministic D cap.'
   }
  }}},
  addEventListener(){}
 };
 const document={documentElement:{},readyState:'loading',addEventListener(){}};
 vm.runInNewContext(source,{window,document,MutationObserver});
 assert.equal(typeof observe,'function');
 observe([{addedNodes:[lens]}]);
 assert.equal(title.textContent,'Why AVOID?');
 assert.match(copy.textContent,/Verified owner concentration/);
 assert.equal(action.textContent,'AVOID TARGET');
 assert.equal(action.dataset.policy,'block');
 assert.equal(lens.dataset.protectionDecision,'avoid');
});

test('late decision lens remains unchanged if no backend investor decision exists',()=>{
 const source=fs.readFileSync(path.join(__dirname,'owner-investor-protection.js'),'utf8');
 const title={textContent:'Why REVIEW?'};
 const lens={
  nodeType:1,dataset:{},
  matches:selector=>selector==='.koschei-decision-lens',
  querySelector:selector=>selector==='.koschei-decision-lens__copy h3'?title:null,
  closest:selector=>selector==='[data-arvis-premium-card]'?card:null
 };
 const card={querySelector:selector=>selector==='.koschei-decision-lens'?lens:null};
 let observe;
 class MutationObserver {
  constructor(callback){observe=callback;}
  observe(){}
 }
 const window={OwnerRadarKit:{lastScan:{investigation_report:{}}},addEventListener(){}};
 const document={documentElement:{},readyState:'loading',addEventListener(){}};
 vm.runInNewContext(source,{window,document,MutationObserver});
 observe([{addedNodes:[lens]}]);
 assert.equal(title.textContent,'Why REVIEW?');
});
