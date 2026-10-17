'use strict';
const assert=require('node:assert/strict');
const fs=require('node:fs');
const path=require('node:path');
const vm=require('node:vm');
const js=fs.readFileSync(path.join(__dirname,'../public/js/owner-authority-source-addresses.js'),'utf8');
const html=fs.readFileSync(path.join(__dirname,'../public/owner-production.html'),'utf8');
assert.match(html,/owner-authority-source-addresses\.js\?v=1/);

class Element{
  constructor(tag){this.tagName=tag;this.children=[];this.textContent='';this.className='';this.id='';this.removed=false;}
  appendChild(child){this.children.push(child);return child;}
  remove(){this.removed=true;}
  querySelector(selector){
    if(selector.startsWith('.'))return this.children.find(x=>x.className===selector.slice(1)&&!x.removed)||null;
    return null;
  }
}
function descendants(node){return[node,...node.children.flatMap(descendants)].filter(x=>!x.removed);}
function run(payload){
  const root=new Element('main');
  const identity=new Element('article');
  identity.id='target-identity-card';
  root.querySelector=selector=>selector==='#target-identity-card'?identity:null;
  const document={
    getElementById(){return null;},
    createElement:tag=>new Element(tag),
    head:new Element('head'),
  };
  let originalRender=0,originalScan=0;
  const ctx={window:{
    OwnerRadarKit:{
      renderUnified(){originalRender++},
      scan:async()=>{originalScan++;return payload;},
    },
  },document};
  vm.runInNewContext(js,ctx,{filename:'owner-authority-source-addresses.js'});
  ctx.window.OwnerRadarKit.renderUnified(root,payload);
  return{identity,ctx,root,renderCount:()=>originalRender,scanCount:()=>originalScan};
}
const mint='AxJkHRY3puoofyj7Pn512M1SGBgWroXMkEdNPQypkzbu';
const freeze='2apBGMsS6ti9RyF5TwQTDswXBWskiJP2LD4cUEDqYJjk';
const payload={investigation_report:{legacy_14_arm_radar:{source_context:{
  mint_authority:mint,freeze_authority:freeze,
  limitations:['Creator attribution is discovery-only; reverify via canonical RPC.'],
}}}};
let out=run(payload);
assert.equal(out.renderCount(),1,'legacy renderer must remain active');
let sections=out.identity.children.filter(x=>x.className==='owner-authority-source'&&!x.removed);
assert.equal(sections.length,1,'source section not attached exactly once');
let strings=descendants(sections[0]).map(x=>x.textContent);
assert.ok(strings.includes(mint),'complete mint pubkey was truncated');
assert.ok(strings.includes(freeze),'complete freeze pubkey was truncated');
assert.ok(strings.some(x=>x.includes('bağımsız RPC doğrulaması değil')),'source assurance caveat missing');
assert.ok(strings.some(x=>x.includes('discovery-only')),'original source limitations were removed');
assert.ok(strings.some(x=>x.includes('güncel kanonik yetki')),'current authority is not established');
out.ctx.window.OwnerRadarKit.renderUnified(out.root,payload);
assert.equal(out.renderCount(),2);
assert.equal(out.identity.children.filter(x=>x.className==='owner-authority-source'&&!x.removed).length,1,'render added duplicate authority sections');

out=run({source_context:{mint_authority:mint,freeze_authority:'<img src=x onerror=alert(1)>'}});
sections=out.identity.children.filter(x=>x.className==='owner-authority-source'&&!x.removed);
assert.equal(sections.length,1);
strings=descendants(sections[0]).map(x=>x.textContent);
assert.ok(strings.some(x=>x.includes('Kaynakta doğrulanabilir adres yok')));
assert.ok(!strings.includes('<img src=x onerror=alert(1)>'),'invalid address injected in source view');

out=run({investigation_report:{source_context:{mint_authority:null,freeze_authority:null}}});
assert.equal(out.identity.children.filter(x=>x.className==='owner-authority-source').length,0,'no source cannot assert revoked status');

out=run({investigation_report:{source_context:{mint_authority:mint,freeze_authority:''}}});
sections=out.identity.children.filter(x=>x.className==='owner-authority-source'&&!x.removed);
assert.equal(sections.length,1,'partial source should expose known address');
strings=descendants(sections[0]).map(x=>x.textContent);
assert.ok(strings.some(x=>x.includes('revoked sonucu çıkarılamaz')),'absent address must not become revoked');
console.log('Owner source authority acceptance passed: exact addresses, discovery boundary, XSS resistance, absence not revoked, idempotent render.');
