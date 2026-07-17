(function(root,factory){
  const api=factory();
  if(typeof module==='object'&&module.exports)module.exports=api;
  root.KoscheiFullScanTruthContract=api;
})(typeof globalThis!=='undefined'?globalThis:this,function(){
  'use strict';
  const arr=value=>Array.isArray(value)?value:[];
  const obj=value=>value&&typeof value==='object'&&!Array.isArray(value)?value:{};
  function moduleExecution(module){
    const signals=obj(module?.signals),status=String(signals.execution_status||'').toLowerCase();
    if(['completed','not_applicable','evidence_pending','source_unavailable','insufficient_evidence'].includes(status))return status;
    if(module?.signed)return'completed';
    return'evidence_pending';
  }
  function moduleTechnicalDetail(module){
    const signals=obj(module?.signals),evidence=arr(module?.evidence).map(value=>String(value||'').trim()).filter(Boolean);
    return evidence[0]||String(signals.reason_code||signals.summary||signals.execution_status||module?.verdict||'').trim();
  }
  function actorDossierEmpty(payload){
    const actor=obj(payload?.actor_investigation),dossier=obj(actor.dossier),track=obj(dossier.track);
    if(String(actor.wallet||'').trim())return false;
    if(arr(dossier.tokens).length||arr(dossier.related_actors).length||arr(dossier.evidence).length)return false;
    return ['created_token_count','dominant_holder_token_count','traded_token_count','related_actor_count','verified_evidence_count','observed_evidence_count'].every(key=>Number(track[key]||0)===0);
  }
  function classifyModules(payload){
    const legacy=obj(payload?.legacy_14_arm_radar),modules=arr(legacy.modules);
    return{
      completed:modules.filter(module=>moduleExecution(module)==='completed'),
      gaps:modules.filter(module=>moduleExecution(module)!=='completed')
    };
  }
  return{moduleExecution,moduleTechnicalDetail,actorDossierEmpty,classifyModules};
});
