(function(root,factory){
  const api=factory();
  if(typeof module==='object'&&module.exports)module.exports=api;
  root.KoscheiPublicScanGuard=api;
})(typeof globalThis!=='undefined'?globalThis:this,function(){
  'use strict';
  const activeByRoot=new WeakMap();
  const norm=value=>String(value||'').trim().toLowerCase();
  function begin(root,target){
    if(!root)throw new Error('scan_result_root_missing');
    const clean=String(target||'').trim();
    if(!clean)throw new Error('scan_target_missing');
    const id=globalThis.crypto?.randomUUID?.()||`public-scan-${Date.now()}-${Math.random().toString(16).slice(2)}`;
    const token={id,target:clean,root};
    activeByRoot.set(root,token);
    root.dataset.activePublicScanId=id;
    root.dataset.activePublicScanTarget=clean;
    return token;
  }
  function isActive(token){return Boolean(token?.root)&&activeByRoot.get(token.root)===token}
  function accept(token,report){
    if(!isActive(token))return{accepted:false,reason:'stale_response'};
    const returned=String(report?.target||report?.mint||'').trim();
    if(returned&&norm(returned)!==norm(token.target)){
      return{accepted:false,reason:'target_mismatch',expected:token.target,returned};
    }
    return{accepted:true,reason:'accepted'};
  }
  function finish(token){
    if(!isActive(token))return false;
    activeByRoot.delete(token.root);
    delete token.root.dataset.activePublicScanId;
    return true;
  }
  return{begin,isActive,accept,finish};
});
