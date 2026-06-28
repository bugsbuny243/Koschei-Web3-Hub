(function(){
  function ready(fn){if(document.readyState==='loading'){document.addEventListener('DOMContentLoaded',fn,{once:true});}else{fn();}}

  var translations={
    'Dashboard':'Panel','Radar':'Radar','Token-2022':'Token-2022','Firewall':'İşlem Güvenliği','Watchlist':'İzleme Listesi','Webhooks':'Webhooklar','Integrate':'Entegrasyon','Plans':'Paketler',
    'Architecture':'Mimari','Developers':'Geliştiriciler','Integration Pilot':'Entegrasyon Pilotu',
    'ARVIS command center':'ARVIS komuta merkezi','Evidence-backed only':'Yalnızca kanıta dayalı','User':'Kullanıcı',
    'ARVIS unified radar':'ARVIS birleşik radarı','One radar. Evidence first.':'Tek radar. Önce kanıt.',
    'Solana is the first live market. ARVIS returns a score only when verified on-chain or claim-surface evidence exists. Missing data never becomes a grade or signed report.':'İlk canlı pazar Solana’dır. ARVIS yalnız doğrulanmış zincir üstü veya claim yüzeyi kanıtı varsa skor üretir. Eksik veri hiçbir zaman nota veya imzalı rapora dönüşmez.',
    'Live Radar':'Canlı Radar','Architecture':'Mimari','Go security services':'Go güvenlik servisleri','Runtime engines':'Çalışan motorlar','Checking…':'Kontrol ediliyor…','Output rule':'Çıktı kuralı','Signed + evidence':'İmzalı + kanıt',
    'Run ARVIS':'ARVIS’i çalıştır','Active Plan':'Aktif Paket','Remaining Outputs':'Kalan Çıktı','Core Status':'Temel Durum','Pipeline':'İşlem hattı','Stream':'Akış','Runtime arms':'Çalışan kanıt kolları','Visible cards':'Görünen kartlar','Processed':'İşlenen','No evidence':'Kanıt yok','Failed':'Başarısız','Last event':'Son olay',
    'Reading entitlement.':'Paket yetkisi okunuyor.','Failed evidence collection is not charged.':'Başarısız kanıt toplama işlemi ücrete tabi değildir.','Loading account access and ARVIS status…':'Hesap erişimi ve ARVIS durumu yükleniyor…',
    'View Plans':'Paketleri Gör','Open Live Radar':'Canlı Radarı Aç','Explore Tools':'Araçları İncele','No active plan':'Aktif paket yok','Choose a plan to unlock customer scans.':'Müşteri taramalarını açmak için bir paket seçin.','Entitlement verified.':'Paket yetkisi doğrulandı.','Get Outputs':'Çıktı Satın Al',
    'Choose a plan to run ARVIS':'ARVIS’i çalıştırmak için paket seçin','The live production radar remains visible, while customer scans, reports, watchlists and alerts require an active package.':'Canlı üretim radarı görüntülenebilir; müşteri taramaları, raporlar, izleme listeleri ve alarmlar için aktif paket gerekir.',
    'No outputs remaining':'Kalan çıktı yok','Your package is active, but a new output allocation is required before another customer scan can run.':'Paketiniz aktif ancak yeni bir müşteri taraması için ek çıktı hakkı gerekir.','Enter a target. A verdict appears only after evidence verification.':'Bir hedef girin. Karar yalnız kanıt doğrulandıktan sonra görünür.',
    'Locked':'Kilitli','Live':'Canlı','Stale':'Güncelliğini yitirmiş','Waiting':'Bekleniyor','verified':'doğrulanmış','verified evidence':'doğrulanmış kanıt','ARVIS engine':'ARVIS motoru','Verified observation':'Doğrulanmış gözlem','Real data unavailable. No output was charged.':'Gerçek veri kullanılamıyor. Çıktı hakkı düşülmedi.','Signed ARVIS verdict':'İmzalı ARVIS kararı','Verified verdict':'Doğrulanmış karar','Vault':'Rapor Kasası','Enter a target.':'Bir hedef girin.','Collecting verified evidence…':'Doğrulanmış kanıt toplanıyor…','Analysis failed.':'Analiz başarısız.','Verified evidence unavailable.':'Doğrulanmış kanıt kullanılamıyor.','No output was charged.':'Çıktı hakkı düşülmedi.','ARVIS response unavailable.':'ARVIS yanıtı kullanılamıyor.'
  };

  function translateString(value){
    var text=String(value||'');
    var trimmed=text.trim();
    if(translations[trimmed]) return text.replace(trimmed,translations[trimmed]);
    if(/^Enter a Solana token, pool, wallet, program, transaction or claim URL$/i.test(trimmed)) return 'Solana token, havuz, cüzdan, program, işlem veya claim URL’si girin';
    return text;
  }

  function translate(root){
    if(!root||root.nodeType!==1)return;
    var walker=document.createTreeWalker(root,NodeFilter.SHOW_TEXT);
    var nodes=[];while(walker.nextNode())nodes.push(walker.currentNode);
    nodes.forEach(function(node){var p=node.parentElement;if(!p||/^(SCRIPT|STYLE|CODE|PRE)$/.test(p.tagName))return;var next=translateString(node.nodeValue);if(next!==node.nodeValue)node.nodeValue=next;});
    root.querySelectorAll('input[placeholder],textarea[placeholder]').forEach(function(el){el.placeholder=translateString(el.placeholder);});
    document.documentElement.lang='tr';
  }

  ready(function(){
    var links=[['/dashboard','Panel'],['/security-radar','Radar'],['/token-2022-scanner','Token-2022'],['/transaction-firewall','İşlem Güvenliği'],['/watchlist','İzleme Listesi'],['/webhooks','Webhooklar'],['/pilot','Entegrasyon'],['/pricing','Paketler']];
    var current=(location.pathname||'/').replace(/\.html$/,'').replace(/\/$/,'')||'/';
    var existing=document.querySelector('.top .nav, header.top nav.nav, nav.top .nav');
    var nav=existing||document.createElement('nav');
    nav.className=(existing?'nav ':'')+'koschei-global-nav';
    nav.setAttribute('aria-label','Ana menü');
    while(nav.firstChild)nav.removeChild(nav.firstChild);
    links.forEach(function(item){var a=document.createElement('a');a.href=item[0];a.textContent=item[1];if(current===item[0])a.setAttribute('aria-current','page');nav.appendChild(a);});
    if(!existing){var top=document.querySelector('header.top,.top');if(top){nav.className+=' detached';top.parentNode.insertBefore(nav,top.nextSibling);}}
    var bottom=document.querySelector('nav.bottom');if(bottom)bottom.remove();
    if(!document.querySelector('.koschei-footer')){var footer=document.createElement('footer');footer.className='koschei-footer';footer.innerHTML='<span>Koschei ARVIS · Solana imza öncesi risk altyapısı</span><span><a href="/architecture">Mimari</a> · <a href="/developers">Geliştiriciler</a> · <a href="/pilot">Entegrasyon Pilotu</a> · <a href="/pricing">Paketler</a></span>';document.body.appendChild(footer);}
    translate(document.body);
    var observer=new MutationObserver(function(records){records.forEach(function(record){record.addedNodes.forEach(function(node){if(node.nodeType===1)translate(node);else if(node.nodeType===3&&node.parentElement){var next=translateString(node.nodeValue);if(next!==node.nodeValue)node.nodeValue=next;}});});});
    observer.observe(document.body,{childList:true,subtree:true,characterData:false});
  });
})();