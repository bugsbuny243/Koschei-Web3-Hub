const {test}=require('node:test');
const assert=require('node:assert/strict');
const fs=require('node:fs');
const vm=require('node:vm');
const path=require('node:path');
const source=fs.readFileSync(path.join(__dirname,'..','crypto-brief.js'),'utf8');
class Element {
 constructor(){this.children=[];this.events={};this.dataset={};this.hidden=true;this.checked=false;this.value='';this.textContent='';}
 append(...nodes){this.children.push(...nodes);}
 replaceChildren(...nodes){this.children=nodes;}
 addEventListener(type,fn){this.events[type]=fn;}
 removeAttribute(name){delete this[name];}
 set innerHTML(_){throw new Error('Publisher news must never enter HTML');}
}
const settle=()=>new Promise(resolve=>setImmediate(resolve));
function harness({loggedIn=true,items=[],configured=true}={}){
 const nodes={};const calls=[];const defaults={networks:[],topics:[],cadence:'daily',timezone:'Europe/Istanbul',quiet_start:23,quiet_end:8};
 const subscriptions=[{channel:'telegram',configured,template_configured:true,state:'disconnected',preferences:defaults}];
 const document={getElementById(id){return nodes[id]||(nodes[id]=new Element());},createElement(){return new Element();},querySelectorAll(){return [];}};
 document.getElementById('briefConsent');
 const auth={init:async()=>{},isLoggedIn:()=>loggedIn,apiCall:async(url,options={})=>{calls.push({url,options});return {ok:true,status:200,json:async()=>url.endsWith('/pair')?{url:'https://t.me/FixtureBot?start=fixture',command:'/start fixture',expires_at:new Date(Date.now()+600000).toISOString()}:{subscriptions}};}};
 vm.runInNewContext(source,{window:{KoscheiAuth:auth,addEventListener(){}},document,URL,Date,setTimeout:()=>1,clearTimeout(){},fetch:async()=>({ok:true,json:async()=>({items,sources:[{id:'fixture',name:'Fixture publisher',feed_url:'https://publisher.example/rss'}],source_status:[]})})});
 return {nodes,calls};
}
test('public news can load without a customer session; connecting requires login',async()=>{
 const h=harness({loggedIn:false});await settle();assert.equal(h.calls.length,0);assert.equal(h.nodes.briefLogin.hidden,false);assert.equal(h.nodes.briefPreferences.hidden,true);assert.match(h.nodes.briefAuthState.textContent,/giriş yapın/);
});
test('a Telegram connection cannot start without explicit consent',async()=>{
 const h=harness();await settle();const pair=h.nodes.briefChannels.children[0].children[2].children[0];await pair.events.click();assert.equal(h.calls.filter(x=>x.url.endsWith('/pair')).length,0);assert.match(h.nodes.briefMessage.textContent,/bildirim iznini/);
});
test('consent saves only Telegram preferences and returns a temporary challenge without browser storage',async()=>{
 const h=harness();await settle();h.nodes.briefConsent.checked=true;const pair=h.nodes.briefChannels.children[0].children[2].children[0];await pair.events.click();const pairCall=h.calls.find(x=>x.url.endsWith('/pair'));assert.ok(pairCall);assert.equal(JSON.parse(pairCall.options.body).consent,true);assert.equal(JSON.parse(pairCall.options.body).channel,'telegram');const preferenceCalls=h.calls.filter(x=>x.url==='/api/customer/crypto-brief'&&x.options.method==='PUT');assert.ok(preferenceCalls.length>=1);assert.ok(preferenceCalls.every(x=>JSON.parse(x.options.body).channel==='telegram'));assert.equal(h.nodes.briefPairing.hidden,false);assert.match(h.nodes.briefPairLink.href,/^https:\/\/t.me\/FixtureBot/);assert.equal(h.nodes.briefPairCommand.textContent,'/start fixture');
});
test('publisher strings remain text and foreign news links are rejected',async()=>{
 const title='<img src=x onerror=alert(1)>';const base={source_id:'fixture',title,published_at:new Date().toISOString(),networks:['solana'],topics:['network']};
 const h=harness({loggedIn:false,items:[{...base,url:'https://publisher.example/news'},{...base,url:'https://evil.example/news'}]});await settle();assert.equal(h.nodes.briefFeed.children.length,1);assert.equal(h.nodes.briefFeed.children[0].children[0].textContent,title);assert.equal(h.nodes.briefFeed.children[0].children[0].rel,'noopener noreferrer');
});
