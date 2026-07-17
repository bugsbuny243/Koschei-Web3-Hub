'use strict';

const guard=require('../public/js/public-scan-request-guard.js');

function root(){return{dataset:{}}}

const resultRoot=root();
const first=guard.begin(resultRoot,'MintOne');
const second=guard.begin(resultRoot,'MintTwo');

const stale=guard.accept(first,{target:'MintOne'});
if(stale.accepted||stale.reason!=='stale_response')throw new Error(`stale response accepted: ${JSON.stringify(stale)}`);
if(guard.isActive(first))throw new Error('superseded request remained active');
if(!guard.isActive(second))throw new Error('latest request is not active');

const mismatch=guard.accept(second,{target:'MintThree'});
if(mismatch.accepted||mismatch.reason!=='target_mismatch'||mismatch.expected!=='MintTwo'||mismatch.returned!=='MintThree'){
  throw new Error(`target mismatch accepted: ${JSON.stringify(mismatch)}`);
}

const accepted=guard.accept(second,{target:' minttwo '});
if(!accepted.accepted)throw new Error(`case-insensitive target match rejected: ${JSON.stringify(accepted)}`);
if(!guard.finish(second))throw new Error('active request could not finish');
if(guard.isActive(second))throw new Error('finished request remained active');
if(guard.finish(first))throw new Error('stale request finished the active slot');

console.log('public scan stale-response contract: ok');
