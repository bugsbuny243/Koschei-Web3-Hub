'use strict';
const assert=require('node:assert/strict');
const fs=require('node:fs');
const path=require('node:path');
const vm=require('node:vm');
const source=fs.readFileSync(path.join(__dirname,'../public/js/owner-decision-persistence-truth.js'),'utf8');
const html=fs.readFileSync(path.join(__dirname,'../public/owner-production.html'),'utf8');
assert.match(html,/owner-decision-persistence-truth\.js\?v=1/);

class E {
  constructor(tag){this.tag=tag;this.className='';this.textContent='';this.dataset={};this.children=[];this.id='';}
  appendChild(child){this.children.push(child);return child;}
  setAttribute(){}
  querySelector(selector){
    const exact=selector.startsWith('.')?selector.slice(1):selector;
    return this.children.find(x=>x.className===exact || x.id===exact)||
      this.children.map(x=>x.querySelector?.(selector)).find(Boolean)||null;
  }
}
function makeLens(){
  const lens=new E('section');lens.className='koschei-decision-lens';
  const title=new E('h3');title.textContent='Why REVIEW?';
  const copy=new E('p');copy.textContent='No deterministic blocking rule fired.';
  const action=new E('span');action.textContent='REVIEW RESULT';action.dataset.policy='review';
  const original=lens.querySelector.bind(lens);
  lens.querySelector=selector=>({
    '.koschei-decision-lens__copy h3':title,
    '.koschei-decision-lens__copy p':copy,
    '.koschei-decision-lens__action':action,
  })[selector]||original(selector);
  return {lens,title,copy,action};
}
function build(){
  const card=new E('article');
  const head=new E('div');head.className='arvis-premium-head';
  head.insertAdjacentElement=(_direction,node)=>card.appendChild(node);
  card.appendChild(head);
  const root=new E('main');
  root.querySelectorAll=selector=>selector==='[data-arvis-premium-card]'?[card]:[];
  const dom=new E('html');
  const styles=new E('head');
  let observerCB=()=>{};
  const document={
    head:styles,documentElement:dom,
    getElementById(){return null;},
    createElement:tag=>new E(tag),
  };
  const oldScan={investigation_report:{investor_protection_decision:{decision:'REVIEW_FIRST'}}};
  let calls=0;
  const window={
    OwnerRadarKit:{
      lastScan:oldScan,
      renderUnified(){calls++;},
      render(){calls++;},
      scan:async()=>oldScan,
    },
  };
  class MutationObserver{
    constructor(cb){observerCB=cb;}
    observe(){}
  }
  vm.runInNewContext(source,{window,document,MutationObserver});
  return {window,card,root,getObserver:()=>observerCB,calls:()=>calls};
}

const example={
  investigation_report:{
    final_verdict:{grade:'D',verdict:'hard_trigger',signed:true},
    final_verdict_persistence:'failed',
    investor_protection_decision:{
      decision:'AVOID',execution_action:'BLOCK',investor_action:'AVOID TARGET',
      summary:'Verified cross-token transfer triggered URD-C006, not evidence of wrongdoing.',
    },
  },
};
const state=build();
const {lens,title,copy,action}=makeLens();
state.card.appendChild(lens);
state.window.OwnerRadarKit.renderUnified(state.root,example);
assert.equal(state.calls(),1,'existing renderer must remain active');
assert.equal(title.textContent,'Why AVOID?');
assert.equal(action.textContent,'AVOID TARGET');
assert.equal(action.dataset.policy,'block');
assert.match(copy.textContent,/URD-C006/);
assert.equal(lens.dataset.protectionDecision,'avoid');
assert.equal(state.window.OwnerRadarKit.lastScan,example,'new result overrides old stale lastScan');
let panel=state.card.querySelector('.owner-verdict-persistence-truth');
assert.ok(panel,'verdict storage state panel not shown');
assert.match(panel.querySelector('.owner-verdict-persistence-state').textContent,/kalıcı verdict kaydı BAŞARISIZ/);
assert.match(panel.querySelector('.owner-verdict-persistence-limit').textContent,/bağımsız doğrulandığı/);
assert.equal(panel.dataset.persistenceRisk,'failure');

const next={
  investigation_report:{
    final_verdict_persistence:'persisted',
    investor_protection_decision:{
      decision:'REVIEW_FIRST',execution_action:'WARN',
      summary:'No blocking grade is claimed for this separate test result.',
    },
  },
};
state.window.OwnerRadarKit.renderUnified(state.root,next);
assert.equal(title.textContent,'Why REVIEW FIRST?','decision must reflect latest payload');
assert.equal(action.dataset.policy,'warn');
assert.equal(state.card.children.filter(x=>x.className==='owner-verdict-persistence-truth').length,1,'no duplicate banner');
assert.match(panel.querySelector('.owner-verdict-persistence-state').textContent,/Kalıcı hüküm kaydı: persisted/);

const late=build();
late.window.OwnerRadarKit.renderUnified(late.root,example);
const newLens=makeLens();
late.card.appendChild(newLens.lens);
late.getObserver()([{addedNodes:[newLens.lens]}]);
assert.equal(newLens.title.textContent,'Why AVOID?','late injection of REVIEW must be reconciled');
assert.equal(newLens.action.dataset.policy,'block');

const unknown=build();
unknown.card.appendChild(makeLens().lens);
unknown.window.OwnerRadarKit.renderUnified(unknown.root,{investigation_report:{final_verdict:{signed:true}}});
assert.match(unknown.card.querySelector('.owner-verdict-persistence-state').textContent,/bağımsız olarak doğrulanmadı/);
console.log('Owner decision+persistence truth passed: AVOID/BLOCK over stale REVIEW, failed storage warning, follow-up scan, late UI injection, unknown persistence.');
