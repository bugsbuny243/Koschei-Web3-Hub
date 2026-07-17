'use strict';

const truth=require('../public/js/full-scan-truth-contract.js');

const emptyActor={actor_investigation:{wallet:'',dossier:{tokens:[],related_actors:[],evidence:[],track:{created_token_count:0,dominant_holder_token_count:0,traded_token_count:0,related_actor_count:0,verified_evidence_count:0,observed_evidence_count:0}}}};
if(!truth.actorDossierEmpty(emptyActor))throw new Error('empty actor dossier was not classified as empty');

const populatedActor={actor_investigation:{wallet:'Wallet111',dossier:{tokens:[],related_actors:[],evidence:[],track:{}}}};
if(truth.actorDossierEmpty(populatedActor))throw new Error('populated actor dossier was hidden');

const payload={legacy_14_arm_radar:{modules:[
  {module_id:'authority',signed:true,signals:{execution_status:'completed',reason_code:'authority_parsed'},evidence:['mint authority revoked']},
  {module_id:'liquidity',signed:false,signals:{execution_status:'evidence_pending',reason_code:'lp_owner_unresolved'},evidence:[]},
  {module_id:'claim',signed:false,signals:{execution_status:'not_applicable',reason_code:'token_target_only'},evidence:[]}
]}};
const classified=truth.classifyModules(payload);
if(classified.completed.length!==1||classified.gaps.length!==2)throw new Error(`collector classification failed: ${JSON.stringify(classified)}`);
if(truth.moduleTechnicalDetail(classified.completed[0])!=='mint authority revoked')throw new Error('real collector evidence was not preferred');
if(truth.moduleTechnicalDetail(classified.gaps[0])!=='lp_owner_unresolved')throw new Error('technical gap code was not preserved');
if(truth.moduleTechnicalDetail({signals:{execution_status:'completed'},evidence:[]})!=='completed')throw new Error('empty collector fallback invented narrative text');

console.log('full-scan truth contract: ok');
