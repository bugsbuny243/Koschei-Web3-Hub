(function(){
  function esc(v){return String(v==null?'':v).replace(/[&<>"']/g,function(c){return {'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[c]})}
  function currentScript(){return document.currentScript||Array.prototype.slice.call(document.getElementsByTagName('script')).filter(function(s){return (s.src||'').indexOf('/widget.js')>=0}).pop()}
  // An unreachable API is UNKNOWN, never a deterministic "A/Low" hash of the address.
  function unavailable(token){return {ok:false,address:token,grade:'—',risk_index:null,risk_level:'unknown',verdict:'Security assessment unavailable. No verified verdict was returned.',signed:false}}
  function verified(data){return data&&data.ok===true&&data.signed===true&&typeof data.signature==='string'&&data.signature.length>0&&typeof data.grade==='string'&&/^[A-F]$/.test(data.grade)&&typeof data.risk_index==='number'&&Number.isFinite(data.risk_index)&&data.risk_index>=0&&data.risk_index<=100&&['critical','high','medium','low'].indexOf(String(data.risk_level||'').toLowerCase())>=0}
  function render(root,data){
    var valid=verified(data),level=valid?String(data.risk_level).toLowerCase():'unknown';
    var labels={critical:'Critical',high:'High',medium:'Medium',low:'Low',unknown:'Unknown'};
    var color=level==='critical'||level==='high'?'#ff4d6d':level==='medium'?'#ffcc66':level==='low'?'#18ffb2':'#9aabb8';
    var grade=valid?data.grade:'—',risk=valid?data.risk_index+'/100':'Unknown';
    var verdict=valid?data.verdict:'Security assessment unavailable. No verified verdict was returned.';
    root.innerHTML='<a href="https://tradepigloball.co/security-ecosystem" target="_blank" rel="noopener" style="text-decoration:none;color:inherit"><div style="border:1px solid rgba(36,234,255,.28);background:#10151e;border-radius:18px;padding:13px 14px;display:grid;gap:9px"><div style="display:flex;align-items:center;justify-content:space-between;gap:12px"><strong style="font-size:13px;color:#cffff9">Koschei Web3 Hub</strong><span style="font-size:11px;border:1px solid '+color+';color:'+color+';border-radius:999px;padding:5px 8px">'+esc(labels[level])+'</span></div><div style="display:flex;align-items:end;gap:12px"><div style="font-size:46px;font-weight:900">'+esc(grade)+'</div><div style="font-size:13px">Risk Index '+esc(risk)+'</div></div><div style="font-size:11px;color:#9aabb8">'+esc(verdict)+'</div></div></a>';
  }
  function endpoint(script,token){
    try{
      var origin=script&&script.getAttribute('data-api-base');
      // Embed clients default to the ORIGIN OF THE SCRIPT rather than the
      // embedding site's origin. Explicit override is permitted via data-api-base.
      if(!origin&&script&&script.src)origin=new URL(script.src,document.baseURI).origin;
      if(!origin)return null;
      var url=new URL('/api/v1/risk/badge',origin);
      if(url.protocol!=='https:'&&url.hostname!=='localhost'&&url.hostname!=='127.0.0.1')return null;
      url.searchParams.set('address',token);
      return url.toString();
    }catch(_){return null}
  }
  var script=currentScript(),token=script&&script.getAttribute('data-token')||'';
  var root=document.createElement('div');root.setAttribute('data-koschei-badge','1');
  root.style.cssText='font-family:Inter,system-ui,sans-serif;display:inline-block;max-width:360px;color:#f4fbff';
  if(script&&script.parentNode)script.parentNode.insertBefore(root,script.nextSibling);
  var api=token?endpoint(script,token):null;
  if(!api){render(root,unavailable(token));return}
  fetch(api,{credentials:'omit'}).then(function(res){return res.ok?res.json():null})
    .then(function(data){render(root,verified(data)?data:unavailable(token))})
    .catch(function(){render(root,unavailable(token))});
})();
