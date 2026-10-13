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
