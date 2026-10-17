'use strict';
const assert=require('node:assert/strict');
const fs=require('node:fs');
const path=require('node:path');
const vm=require('node:vm');

const code=fs.readFileSync(path.join(__dirname,'../public/js/owner-investigation-ux.js'),'utf8');

function renderLens(report,{policy='review',confidence='medium',grade='D'}={}){
  let domReady=()=>{};
  let root;
  class Element{
    constructor(tag){this.tag=tag;this.className='';this.textContent='';this.children=[];this.dataset={};this.attrs={};this.classList={add(){},contains(){return false},toggle(){}};}
    append(...children){this.children.push(...children);}
    appendChild(child){this.children.push(child);return child;}
    setAttribute(name,value){this.attrs[name]=value;}
    insertAdjacentElement(_position,node){this.inserted=node;}
    querySelector(selector){
      if(selector==='.arvis-premium-head')return selector&&this.head||null;
      return null;
    }
  }
  const head=new Element('header');
  const chip=x=>({textContent:x});
  const card={
    dataset:{},classList:{add(){}},head,
    querySelector(selector){
      if(selector==='.arvis-premium-head')return head;
      if(selector==='.arvis-grade-orb strong')return{textContent:grade};
      if(selector==='.koschei-decision-lens')return head.inserted||null;
      return null;
    },
    querySelectorAll(selector){
      if(selector==='.arvis-chip')return [
        chip('Policy '+policy),chip('Confidence '+confidence),chip('Signed verdict')
      ];
      if(selector==='.arvis-rule')return Array.from({length:3},()=>({}));
      return [];
    },
  };
  const document={
    readyState:'loading',documentElement:{},
    createElement:tag=>new Element(tag),
    addEventListener(name,fn){if(name==='DOMContentLoaded')domReady=fn;},
    querySelectorAll(selector){return selector==='[data-arvis-premium-card]'?[card]:[];},
  };
  let invoke=()=>{};
  const win={
    OwnerRadarKit:{lastScan:{investigation_report:report}},
    addEventListener(name,fn){if(name==='koschei:investigation-rendered')invoke=fn;},
  };
  class MutationObserver{constructor(fn){this.callback=fn;}observe(){}}
  vm.runInNewContext(code,{window:win,document,MutationObserver},{filename:'owner-investigation-ux.js'});
  domReady();
  invoke();
  const lens=head.inserted;
  assert.ok(lens,'decision lens not mounted');
  const top=lens.children[0];
  const copy=top.children[0];
  return {
    title:copy.children[1].textContent,
    description:copy.children[2].textContent,
    action:top.children[1].textContent,
    policy:top.children[1].dataset.policy,
    stat: lens.children[1].children.map(n=>n.children[1].textContent),
  };
}

const avoid={
  final_verdict:{grade:'D',signed:true,verdict:'hard_trigger'},
  investor_protection_decision:{
    decision:'AVOID',execution_action:'BLOCK',investor_action:'AVOID_TARGET',
    summary:'Grade D follows a verified cross-token actor hard-cap rule.',
  },
};
let result=renderLens(avoid);
assert.equal(result.title,'Why AVOID?');
assert.equal(result.action,'AVOID TARGET');
assert.equal(result.policy,'block');
assert.match(result.description,/verified cross-token actor/);
assert.doesNotMatch(result.description,/No deterministic blocking rule fired/);
assert.ok(result.stat.includes('SIGNED'),'signed only describes verdict envelope');
assert.ok(result.stat.includes('MEDIUM'),'confidence kept distinct from signature');

result=renderLens({final_verdict:{grade:'D',verdict:'hard_trigger'}});
assert.equal(result.title,'Why ACTION UNRESOLVED?');
assert.equal(result.action,'PROTECTION DECISION UNAVAILABLE');
assert.match(result.description,/hard-trigger verdict/i);
assert.doesNotMatch(result.description,/No deterministic blocking rule fired/);

result=renderLens({final_verdict:{grade:'D',verdict:'evidence_bounded'}});
assert.equal(result.title,'Why REVIEW?');
assert.match(result.description,/contextual REVIEW label/);
assert.doesNotMatch(result.description,/No deterministic blocking rule fired/);

result=renderLens({final_verdict:{grade:'A',verdict:'clear'}},{policy:'allow',grade:'A',confidence:'high'});
assert.match(result.description,/No deterministic blocking rule fired/);
assert.equal(result.action,'NO BLOCK RULE FIRED');

result=renderLens({investor_protection_decision:{
  decision:'NOT_CLEARED',execution_action:'WITHHOLD',
}});
assert.equal(result.title,'Why NOT CLEARED?');
assert.equal(result.policy,'withhold');
assert.match(result.description,/not cleared/i);

console.log('Owner decision-lens regression passed: server AVOID dominates REVIEW, missing action fails closed, signed/confidence distinct.');
