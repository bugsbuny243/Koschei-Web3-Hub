const {test}=require('node:test');
const assert=require('node:assert/strict');
const api=require('./arvis-premium-contract.js');
for(const [name,payload,known,is2022] of [
 ['missing',{},false,false],
 ['classic ID',{token_program:'TokenkegQfeZyiNwAJbNbGKPFXCWuBvf9Ss623VQ5DA'},true,false],
 ['Token2022 ID',{token_program:'TokenzQdBNbLqP5VEhdkAS6EPFLC1PHnBqCXEpPxuEb'},true,true],
 ['unknown program',{token_program:'unknown'},false,false],
 ['malformed signal',{token_2022:{}},false,false],
 ['false signal',{token_2022:false},true,false],
 ['conflicting evidence',{token_2022:false,token_program:'TokenzQdBNbLqP5VEhdkAS6EPFLC1PHnBqCXEpPxuEb'},false,false],
]) test(name,()=>{
 const model=api.normalize(payload);
 assert.equal(model.token2022Known,known);
 assert.equal(model.isToken2022,is2022);
 const html=api.renderPremiumCard(payload);
 if(!known){assert.match(html,/EVIDENCE MISSING/);assert.doesNotMatch(html,/Classic SPL Token program/);}
 if(is2022){assert.doesNotMatch(html,/Classic SPL Token program/);assert.equal(model.extensionUnresolved,true);}
});

test('classic program needs independently verified mint owner',()=>{
 const program='TokenkegQfeZyiNwAJbNbGKPFXCWuBvf9Ss623VQ5DA';
 for(const payload of [{token_2022:false},{token_program:program},{mint_owner_program:program,mint_owner_program_evidence_status:'observed'}]){
  assert.equal(api.normalize(payload).classicVerified,false);
  assert.match(api.renderPremiumCard(payload),/EVIDENCE MISSING/);
 }
 const payload={mint_owner_program:program,mint_owner_program_evidence_status:'verified'};
 assert.equal(api.normalize(payload).classicVerified,true);
 assert.match(api.renderPremiumCard(payload),/Classic SPL Token program/);
});

test('canonical verified PYUSD extensions populate premium gate without legacy extension rows',()=>{
 const payload={investigation_report:{
  target:'2b1kV6DkPAnxd5ixfnxCpjxmKwqjjaYmCZfHsFu24GXo',
  mint_owner_program:'TokenzQdBNbLqP5VEhdkAS6EPFLC1PHnBqCXEpPxuEb',
  mint_owner_program_evidence_status:'verified',
  token_2022:true,
  token_2022_extension_evidence:{
   status:'resolved',evidence_complete:true,extension_count:2,
   extension_names:['permanentDelegate','transferHook']
  },
  final_verdict:{grade:'D',signed:true,verdict:'hard_trigger'}
 }};
 const model=api.normalize(payload),html=api.renderPremiumCard(payload);
 assert.equal(model.token2022Known,true);
 assert.equal(model.isToken2022,true);
 assert.equal(model.extensionUnresolved,false);
 assert.deepEqual(model.extensionRows.map(x=>x.name),['permanentDelegate','transferHook']);
 assert.match(html,/permanentDelegate/);
 assert.match(html,/transferHook/);
 assert.doesNotMatch(html,/extension list was not resolved|extensions were not completely resolved/);
});

test('canonical incomplete/mismatched extension evidence cannot borrow legacy rows to claim resolution',()=>{
 const base={
  mint_owner_program:'TokenzQdBNbLqP5VEhdkAS6EPFLC1PHnBqCXEpPxuEb',
  mint_owner_program_evidence_status:'verified',token_2022:true,
  token_2022_extension_evidence:{status:'resolved',evidence_complete:true,extension_count:2,extension_names:['transferHook']}
 };
 for(const override of [
  {},
  {token_2022_extension_evidence:{status:'token_2022_extensions_unresolved',evidence_complete:false,extension_count:1,extension_names:['transferHook']}},
  {mint_owner_program_evidence_status:'unverified'},
  {token_2022_extension_evidence:{status:'resolved',evidence_complete:true,extension_count:1,extension_names:['']}}
 ]){
  const payload={investigation_report:{...base,...override},extensions:[{name:'transferHook',severity:'info'}]};
  const model=api.normalize(payload),html=api.renderPremiumCard(payload);
  assert.equal(model.extensionUnresolved,true,JSON.stringify(override));
  assert.match(html,/WITHHOLD/);
 }
});

test('verified resolved_empty mint renders zero extensions, not an unresolved error',()=>{
 const payload={investigation_report:{
  mint_owner_program:'TokenzQdBNbLqP5VEhdkAS6EPFLC1PHnBqCXEpPxuEb',
  mint_owner_program_evidence_status:'verified',
  token_2022_extension_evidence:{status:'resolved_empty',evidence_complete:true,extension_count:0,extension_names:[]}
 }};
 const model=api.normalize(payload),html=api.renderPremiumCard(payload);
 assert.equal(model.extensionUnresolved,false);
 assert.equal(model.extensionRows.length,0);
 assert.match(html,/VERIFIED EMPTY/);
 assert.doesNotMatch(html,/extension list was not resolved/);
});
