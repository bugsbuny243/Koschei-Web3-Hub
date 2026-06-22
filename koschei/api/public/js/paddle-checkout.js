(async()=>{
  const state=document.getElementById('state');
  const fail=message=>{state.textContent=message;state.classList.add('bad')};
  try{
    const response=await fetch('/api/paddle/public-config',{credentials:'same-origin',cache:'no-store'});
    const config=await response.json().catch(()=>({}));
    if(!response.ok||!config.ok)throw new Error(config?.paddle?.missing_fields?.length?`Eksik Paddle ayarları: ${config.paddle.missing_fields.join(', ')}`:'Paddle checkout hazır değil.');
    if(!window.Paddle)throw new Error('Paddle.js yüklenemedi.');
    if(config.environment==='sandbox'&&window.Paddle.Environment?.set)window.Paddle.Environment.set('sandbox');
    window.Paddle.Initialize({
      token:config.client_token,
      checkout:{settings:{displayMode:'overlay',theme:'dark',locale:'tr',allowLogout:false,successUrl:config.success_url}},
      eventCallback:event=>{
        if(event?.name==='checkout.completed'){
          state.textContent='Ödeme tamamlandı. Paketiniz webhook doğrulamasından sonra otomatik aktive ediliyor…';
          setTimeout(()=>location.href=config.success_url||'/dashboard?payment=paddle_success',1200);
        }
        if(event?.name==='checkout.closed'&&!location.search.includes('_ptxn='))state.textContent='Checkout kapatıldı.';
      }
    });
    const params=new URLSearchParams(location.search);
    const transactionId=params.get('_ptxn')||params.get('transaction_id');
    state.textContent=transactionId?'Paddle ödeme penceresi açılıyor…':'Geçerli Paddle transaction bağlantısı bekleniyor.';
    if(transactionId&&!params.has('_ptxn'))window.Paddle.Checkout.open({transactionId});
  }catch(error){fail(error?.message||'Paddle checkout başlatılamadı.');}
})();
