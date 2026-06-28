const PremiumModules = (() => {
  const safe = 'Özel anahtar veya seed phrase girmeyin. Yalnızca herkese açık cüzdan, token, işlem, proje, depo ve sosyal verileri kullanın. Koschei salt okunur istihbarat sağlar; finansal, hukuki, yatırım veya güvenlik tavsiyesi değildir.';
  const positioning = 'Koschei; işlem istihbaratı, cüzdan istihbaratı, token analizi, proje istihbaratı, risk analizi, hibe hazırlığı ve ilişki haritalamayı tek bir Solana/Web3 istihbarat akışında birleştirir.';
  const $ = id => document.getElementById(id);
  const esc = value => String(value ?? '').replace(/[&<>"']/g, char => ({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[char]));

  const configs = {
    chains: {
      title: 'Zincir Sağlığı',
      badge: ['ZİNCİR OPERASYONLARI', 'HESAP ERİŞİMİ'],
      desc: 'Solana ve bağlı veri sağlayıcılarının son sağlık kayıtlarını görüntüleyin. Sonuçlar üretim veritabanındaki gerçek kontrollerden okunur.',
      endpoint: '/api/web3/health/logs', method: 'GET', button: 'Sağlık kayıtlarını yenile', inputLabel: 'Canlı zincir sağlık kayıtları', fields: [],
      normalize: data => ({kayıtlar: data.logs || [], toplam_kayıt: (data.logs || []).length}),
      next: [['Canlı Radarı Aç','/security-radar'], ['Mimariyi İncele','/architecture']]
    },
    metadata: {
      title: 'Metadata Stüdyosu',
      badge: ['PROJE İSTİHBARATI', 'HESAP ERİŞİMİ'],
      desc: 'Proje, token, NFT, hibe ve kamu yararı başvuruları için üretim metadata taslağı ve güvenlik inceleme notları hazırlayın.',
      endpoint: '/api/metadata/generate', method: 'POST', button: 'Metadata oluştur', inputLabel: 'Proje bilgileri',
      fields: [
        ['asset_name','Proje adı','text','Proje adı'],
        ['description','Açıklama','textarea','Gerçek projeyi ve doğrulanabilir kanıtları anlatın'],
        ['asset_type','Kategori','select','builder_tool|token|nft|grant_project|public_good|dao_tool'],
        ['ecosystem','Zincir / ekosistem','text','Solana'],
        ['website','Web sitesi','text','https://'],
        ['socials','Sosyal bağlantılar','textarea','X, Discord, GitHub ve diğer herkese açık bağlantılar']
      ],
      build: f => ({asset_name:f.asset_name, description:f.description, asset_type:f.asset_type, traits:`ecosystem:${f.ecosystem}; website:${f.website}; socials:${f.socials}`}),
      next: [['Proje Radarını Aç','/project-radar'], ['Paketleri Gör','/pricing']]
    },
    tx: {
      title:'İşlem Çözücü', badge:['İŞLEM İSTİHBARATI','PAKET ERİŞİMİ'],
      desc:'Açık Solana işlem imzasını işlem amacı, ilgili programlar, risk bayrakları, kanıt notları ve önerilen sonraki kontroller olarak çözün.',
      endpoint:'/api/v1/unified/analyze', method:'POST', button:'İşlemi çöz', inputLabel:'İşlem imzası',
      fields:[['sig','İşlem imzası','text','Solana işlem imzasını yapıştırın'],['network','Ağ','select','solana-mainnet|solana-devnet|solana-testnet']],
      build:f=>({input:f.sig, context:{input_type:'transaction'}, network:f.network}), next:[['Cüzdan Analizini Başlat','/wallet-score'],['Risk Analizi Başlat','/risk']]
    },
    txpro: {
      title:'Gelişmiş İşlem Çözücü', badge:['İŞLEM İSTİHBARATI','PAKET ERİŞİMİ'],
      desc:'Açık işlem hash’i veya Solana imzasını amaç, varlıklar, risk ipuçları, kanıt notları ve takip analizi rotalarına çözün.',
      endpoint:'/api/v1/unified/analyze', method:'POST', button:'İşlemi çöz', inputLabel:'İşlem imzası veya hash',
      fields:[['tx_hash','İşlem hash’i / imzası','text','0x… veya Solana imzası'],['chain','Zincir','select','solana|ethereum|base|arbitrum|polygon|optimism'],['network','Ağ','text','mainnet']],
      build:f=>({input:f.tx_hash, context:{input_type:'transaction'}, network:f.network}), next:[['Cüzdan Analizini Başlat','/wallet-score'],['Risk Analizi Başlat','/risk']]
    },
    wallet: {
      title:'Cüzdan Skoru', badge:['CÜZDAN İSTİHBARATI','PAKET ERİŞİMİ'],
      desc:'Açık Solana cüzdanını aktivite, güncellik, hata oranı ve bakiye sinyalleriyle skorlayın.',
      endpoint:'/api/v1/unified/analyze', method:'POST', button:'Cüzdan analizini başlat', inputLabel:'Cüzdan adresi',
      fields:[['address','Cüzdan adresi','text','Herkese açık Solana cüzdan adresi'],['network','Ağ','select','solana-mainnet|solana-devnet|solana-testnet']],
      build:f=>({input:f.address, context:{input_type:'wallet'}, network:f.network}), next:[['Portföyü İzle','/portfolio'],['Risk Analizi Başlat','/risk']]
    },
    token: {
      title:'Token Tarayıcısı', badge:['TOKEN İSTİHBARATI','PAKET ERİŞİMİ'],
      desc:'Solana token mint adresini arz, yetki, freeze, holder yoğunluğu ve kanıt temelli risk göstergeleri açısından analiz edin.',
      endpoint:'/api/v1/unified/analyze', method:'POST', button:'Tokenı analiz et', inputLabel:'Token mint adresi',
      fields:[['mint','Token mint adresi','text','Herkese açık Solana token mint adresi'],['network','Ağ','select','solana-mainnet|solana-devnet|solana-testnet']],
      build:f=>({input:f.mint, context:{input_type:'token'}, network:f.network}), next:[['Risk Analizi Başlat','/risk'],['İstihbarat Grafını Aç','/graph']]
    },
    portfolio: {
      title:'Portföy İzleyici', badge:['PORTFÖY İSTİHBARATI','PAKET ERİŞİMİ'],
      desc:'En fazla on Solana cüzdanındaki açık SOL bakiyelerini, token hesaplarını ve son etkinliği izleyin.',
      endpoint:'/api/v1/unified/analyze', method:'POST', button:'Portföyü izle', inputLabel:'Cüzdan adresleri',
      fields:[['addresses','Cüzdan adresleri','textarea','Her satıra bir herkese açık Solana cüzdan adresi'],['network','Ağ','select','solana-mainnet|solana-devnet|solana-testnet']],
      build:f=>({input:String(f.addresses||'').split(/\n|,/).map(x=>x.trim()).filter(Boolean)[0]||'', context:{input_type:'wallet'}, network:f.network}), next:[['Cüzdan Analizini Başlat','/wallet-score'],['Risk Analizi Başlat','/risk']]
    },
    risk: {
      title:'Risk Tarayıcısı', badge:['RİSK ANALİZİ','PAKET ERİŞİMİ'],
      desc:'Cüzdan, token, işlem, kontrat veya proje için kanıt temelli risk değerlendirmesi çalıştırın.',
      endpoint:'/api/v1/unified/analyze', method:'POST', button:'Risk analizini başlat', inputLabel:'Hedef',
      fields:[['target','Cüzdan / token / işlem / proje','text','Herkese açık adres, mint, hash veya proje adı'],['target_type','Hedef türü','select','wallet|token|tx|project|contract'],['chain','Zincir','text','solana'],['network','Ağ','text','mainnet'],['notes','Kanıt notları','textarea','Explorer bağlantıları, depo, web sitesi, bilinen olay veya gözlenen davranış']],
      build:f=>({input:f.target, context:{input_type:f.target_type}, network:f.network, notes:f.notes}), next:[['İşlemi Çöz','/tx-decoder'],['Tokenı Analiz Et','/token-scanner'],['İstihbarat Grafını Aç','/graph']]
    },
    funding: {
      title:'Hibe Hazırlığı ve Başvuru Asistanı', badge:['HİBE İSTİHBARATI','PAKET ERİŞİMİ'],
      desc:'Problem tanımı, Solana ekosistem değeri, mimari, kilometre taşları, bütçe mantığı, kamu yararı, traction kanıtı ve risklerle başvuru materyali hazırlayın.',
      endpoint:'/api/v1/unified/analyze', method:'POST', button:'Hibe hazırlığı oluştur', inputLabel:'Proje bilgileri',
      fields:[['project_name','Proje adı','text','Proje adı'],['ecosystem','Ekosistem','text','Solana'],['project_category','Proje kategorisi','select','security|developer tool|public good|infrastructure|payments|DePIN|AI agent|consumer app|gaming|education'],['short_description','Problem ve çözüm','textarea','Gerçek problemi, çözümünüzü ve mevcut kanıtları anlatın'],['requested_amount','Talep edilen bütçe','text','$25,000'],['milestone_count','Kilometre taşı sayısı','select','3|4|5|6'],['target_users','Hedef kullanıcılar','textarea','Geliştiriciler, hibe başvuranları, ekosistem ekipleri, değerlendiriciler…'],['impact','Açık kaynak / kamu yararı yönü','textarea','Depo, dokümanlar, açık paneller, yeniden kullanılabilir API’ler…'],['traction','Traction / kanıt kontrol listesi','textarea','Kullanıcılar, commitler, dağıtımlar, ortaklar, metrikler, önceki hibeler veya pilotlar…']],
      build:f=>({input:f.project_name||f.short_description, context:{input_type:'question'}, notes:`${f.ecosystem} / ${f.project_category} için hibe hazırlığı. ${f.short_description} Talep: ${f.requested_amount}. Hedef kullanıcılar: ${f.target_users}\nKamu yararı: ${f.impact}\nTraction/kanıt: ${f.traction}`}), next:[['Proje Radarını Aç','/project-radar'],['Paketleri Gör','/pricing']]
    },
    projectRadar: {
      title:'Proje Radarı', badge:['PROJE İSTİHBARATI','PAKET ERİŞİMİ'],
      desc:'Bir projeyi açık URL’ler, GitHub, ekosistem uyumu, güvenilirlik sinyalleri, kanıt noktaları ve hazırlık boşluklarıyla değerlendirin.',
      endpoint:'/api/v1/unified/analyze', method:'POST', button:'Proje radarını çalıştır', inputLabel:'Proje bilgileri',
      fields:[['project_name','Proje adı','text','Proje adı'],['website_url','Web sitesi URL’si','text','https://'],['twitter_handle','X / Twitter hesabı','text','@proje'],['github_url','GitHub URL’si','text','https://github.com/...'],['token_mint_address','Token mint adresi','text','İsteğe bağlı herkese açık token mint adresi'],['public_wallet_address','Herkese açık cüzdan adresi','text','İsteğe bağlı hazine veya deployer cüzdanı'],['ecosystem','Ekosistem','text','Solana'],['category','Kategori','select','security|developer tool|public good|infrastructure|payments|DePIN|AI agent|token|NFT|gaming|consumer app|education'],['description','Proje özeti','textarea','Ürünü, kullanıcıları, mimariyi ve Solana için önemini anlatın'],['known_traction','Traction / kanıtlar','textarea','Kullanıcılar, commitler, dağıtımlar, gelir, hibeler, pilotlar, entegrasyonlar'],['notes','Değerlendirici notları','textarea','Bilinen riskler, eksik kanıtlar, depo/lisans durumu veya sorular']],
      build:f=>({input:f.website_url||f.project_name, context:{input_type:'project', category:f.category}, notes:[f.description,f.known_traction,f.notes].filter(Boolean).join('\n')}), next:[['Hibe Hazırlığı Oluştur','/funding-assistant'],['Risk Analizi Başlat','/risk']]
    },
    graph: {
      title:'İstihbarat Grafı', badge:['MERKEZİ İSTİHBARAT','STUDIO ERİŞİMİ'],
      desc:'Gerçek izleme listesi veya girilen açık adres verilerinden cüzdan, token, işlem, proje, risk sinyali ve sybil bağlantılarını graf görünümünde oluşturun.',
      endpoint:'/api/v1/unified/analyze', method:'POST', button:'İstihbarat grafını oluştur', inputLabel:'Graf kaynağı',
      fields:[['source_id','İzleme listesi kaynak kimliği','text','Hesabınızdaki isteğe bağlı kaynak kimliği'],['address','Herkese açık cüzdan veya kontrat','text','Kaynak kimliği yoksa açık adres'],['chain','Zincir','text','solana'],['network','Ağ','text','mainnet']],
      build:f=>({input:f.address||f.source_id, context:{input_type:f.address?'wallet':'question'}, network:f.network}), next:[['Cüzdan Analizini Başlat','/wallet-score'],['Sybil Kontrolünü Başlat','/sybil-check'],['Risk Analizi Başlat','/risk']]
    },
    sybil: {
      title:'Sybil Kontrolü', badge:['SYBIL İSTİHBARATI','STUDIO ERİŞİMİ'],
      desc:'Cüzdan, proje veya açık konuyu desteklenen cluster göstergeleri, tekrar eden örüntüler, ortak kaynak bağlantıları ve kanıtlarla değerlendirin.',
      endpoint:'/api/v1/unified/analyze', method:'POST', button:'Sybil kontrolünü başlat', inputLabel:'Konu',
      fields:[['subject','Cüzdan, proje veya açık tanımlayıcı','text','Herkese açık cüzdan, proje adı veya tanımlayıcı'],['check_type','Kontrol türü','select','grant|bounty|community|airdrop|custom']],
      build:f=>({input:f.subject, context:{input_type:'wallet'}, notes:f.check_type}), next:[['İstihbarat Grafını Aç','/graph'],['Hibe Hazırlığı Oluştur','/funding-assistant'],['Risk Analizi Başlat','/risk']]
    }
  };

  const keyLabels = {
    ok:'Durum', message:'Mesaj', action:'Önerilen işlem', logs:'Kayıtlar', chain:'Zincir', network:'Ağ', provider:'Sağlayıcı', healthy:'Sağlıklı', result:'Sonuç', error:'Hata', checked_at:'Kontrol zamanı',
    transaction_purpose:'İşlem amacı', involved_wallets_or_programs:'İlgili cüzdanlar veya programlar', risk_flags:'Risk işaretleri', evidence_notes:'Kanıt notları', suggested_next_checks:'Önerilen sonraki kontroller',
    score_breakdown:'Skor kırılımı', trust_level:'Güven seviyesi', active_days:'Aktif günler', balance:'Bakiye', red_flags:'Kırmızı bayraklar', evidence:'Kanıt', suggested_next_analysis:'Önerilen sonraki analiz',
    metadata_security_checks:'Metadata güvenlik kontrolleri', mint_authority:'Mint yetkisi', freeze_authority:'Freeze yetkisi', supply:'Arz', decimals:'Ondalık', authority_notes:'Yetki notları',
    risk_exposure:'Risk maruziyeti', suspicious_assets:'Şüpheli varlıklar', recommended_checks:'Önerilen kontroller', sections:'Bölümler', graph_scope:'Graf kapsamı', empty_state:'Boş durum',
    cluster_indicators:'Küme göstergeleri', confidence:'Güven', recommended_action:'Önerilen işlem', limitation:'Sınırlama', grant_investor_readiness:'Hibe / yatırımcı hazırlığı', suggested_improvements:'Önerilen iyileştirmeler'
  };

  function nav() {
    return '<nav class="nav"><a class="nav-logo" href="/">K Koschei</a><div class="nav-links"><a class="nav-link" href="/dashboard">Panel</a><a class="nav-link" href="/pricing">Paketler</a><a class="nav-link" href="/reports">Raporlar</a><a class="nav-link" href="/account">Hesap</a></div></nav>';
  }

  function optionLabel(value) {
    const labels = {wallet:'Cüzdan',token:'Token',tx:'İşlem',project:'Proje',contract:'Kontrat',grant:'Hibe',bounty:'Ödül programı',community:'Topluluk',airdrop:'Airdrop',custom:'Özel',security:'Güvenlik','developer tool':'Geliştirici aracı','public good':'Kamu yararı',infrastructure:'Altyapı',payments:'Ödemeler','AI agent':'Yapay zekâ ajanı','consumer app':'Tüketici uygulaması',gaming:'Oyun',education:'Eğitim',builder_tool:'Geliştirici aracı',grant_project:'Hibe projesi',public_good:'Kamu yararı',dao_tool:'DAO aracı'};
    return labels[value] || value;
  }

  function field([name,label,type,hint]) {
    if (type === 'select') {
      return `<label class="form-label">${esc(label)}<select class="form-input" name="${esc(name)}">${String(hint||'').split('|').filter(Boolean).map(x=>`<option value="${esc(x)}">${esc(optionLabel(x))}</option>`).join('')}</select></label>`;
    }
    if (type === 'textarea') return `<label class="form-label">${esc(label)}<textarea class="form-input" name="${esc(name)}" placeholder="${esc(hint||'')}"></textarea></label>`;
    return `<label class="form-label">${esc(label)}<input class="form-input" name="${esc(name)}" placeholder="${esc(hint||'')}"></label>`;
  }

  function formValues(form) { return Object.fromEntries(new FormData(form)); }
  function nice(key) { return keyLabels[key] || String(key).replaceAll('_',' ').replace(/\b\w/g, c=>c.toUpperCase()); }

  function normalizeResult(key, payload) {
    const d = payload?.data || payload || {};
    if (key === 'txpro' || key === 'tx') return {...d, transaction_purpose:d.human_summary || d.summary || d.explanation || 'İşlem, mevcut açık verilerden çözüldü.', involved_wallets_or_programs:d.programs || [d.from, d.to].filter(Boolean), risk_flags:d.risk_hints || (d.risk_reason ? [d.risk_reason] : []), evidence_notes:['Sağlayıcı yanıtı ve backend tarafından çözülen işlem alanları.'], suggested_next_checks:['İlgili cüzdanları skorla','Risk analizini başlat','İlgili varlıkları istihbarat grafında aç']};
    if (key === 'wallet') return {...d, score_breakdown:{activity:d.tx_count || d.TxCount || 'yanıtta mevcut', trust_level:d.level || d.label, active_days:d.active_days, balance:d.balance_sol}, red_flags:(Number(d.score) < 40 ? ['Mevcut etkinlik sinyallerine göre düşük skor'] : []), evidence:['Herkese açık Solana RPC hesap, bakiye ve imza geçmişi sinyalleri.'], suggested_next_analysis:['Portföy maruziyetini izle','Risk analizini başlat','İstihbarat grafını aç']};
    if (key === 'token') return {...d, metadata_security_checks:{mint_authority:d.mint_authority || 'devre dışı veya kullanılamıyor', freeze_authority:d.freeze_authority || 'devre dışı veya kullanılamıyor', supply:d.supply, decimals:d.decimals}, risk_flags:d.findings || [], authority_notes:['Yetki alanları mevcut olduğunda herkese açık Solana token hesap verilerinden okunur.'], suggested_next_analysis:['Risk analizini başlat','İstihbarat grafını aç']};
    if (key === 'portfolio') return {...d, risk_exposure:'Düşük likiditeli veya bilinmeyen token hesaplarını manuel inceleyin; uç nokta açık bakiye ve token hesap sayılarını raporlar.', suspicious_assets:'Bu uç nokta kanıtsız şüpheli varlık sınıflandırması yapmaz.', recommended_checks:['Her cüzdanda cüzdan skoru çalıştır','Yüksek maruziyetli cüzdan veya tokenlarda risk analizi başlat']};
    if (key === 'funding') return {...d, sections:['problem tanımı','Solana ekosistem değeri','teknik mimari','kilometre taşları','bütçe mantığı','açık kaynak / kamu yararı','traction ve kanıt kontrol listesi','riskler ve azaltma planı','son başvuru taslağı']};
    if (key === 'graph') return {...d, graph_scope:['cüzdanlar','tokenlar','işlemler','projeler','risk sinyalleri','sybil bağlantıları','hibe / proje ilişkileri'], empty_state:((d.nodes||[]).length===0 && (d.edges||[]).length===0) ? 'Henüz graf ilişkisi bulunamadı. Gerçek modül analizi, izleme listesi kaynakları veya gönderilen açık adreslerden graf verisi oluşturun.' : undefined};
    if (key === 'sybil') return {...d, cluster_indicators:d.signals || [], evidence:d.signals || ['Gönderilen açık konu mevcut hesap verileriyle kontrol edildi.'], confidence:d.preliminary ? 'orta' : 'yüksek', recommended_action:d.recommendation || 'Herhangi bir işlemden önce manuel inceleme önerilir.', limitation:'Kanıt olmadan sybil suçlaması yapılmaz; bunu inceleme kuyruğu sinyali olarak kullanın.'};
    if (key === 'projectRadar') return {...d, grant_investor_readiness:d.opportunity_score || d.public_good_score || 'Hazırlık çıktısını ve eksik kanıt noktalarını inceleyin.', suggested_improvements:d.what_to_check_next || d.manual_review_notes || []};
    return d;
  }

  function renderValue(value) {
    if (value === null || value === undefined || value === '') return '—';
    if (typeof value === 'boolean') return value ? 'Evet' : 'Hayır';
    if (typeof value === 'object') return JSON.stringify(value, null, 2);
    return String(value);
  }

  function render(data, status='Analiz tamamlandı.') {
    const result = $('result');
    if (!result) return;
    const entries = Object.entries(data || {}).filter(([key]) => !['ok','raw','decoded'].includes(key));
    result.innerHTML = `<div class="result-head"><h2>${esc(status)}</h2><button class="btn btn-ghost" id="copyResult">JSON kopyala</button></div><div class="result-grid">${entries.map(([key,value])=>`<div class="result-item"><b>${esc(nice(key))}</b><span>${esc(renderValue(value))}</span></div>`).join('')}</div><details class="raw-json"><summary>Kanıt JSON</summary><pre>${esc(JSON.stringify(data || {}, null, 2))}</pre></details>`;
    const btn = $('copyResult');
    if (btn) btn.onclick = () => navigator.clipboard?.writeText(JSON.stringify(data || {}, null, 2));
  }

  function showPackageRequired() {
    render({access:'Aktif Koschei paketi gerekli.', action:'Devam etmek için Starter, Builder veya Studio seçin.'}, 'Aktif Koschei paketi gerekli.');
    $('result')?.insertAdjacentHTML('beforeend','<div class="premium-actions"><a class="btn btn-primary" href="/pricing">Paketleri Gör</a><a class="btn btn-ghost" href="/account">Hesabı Kontrol Et</a></div>');
  }

  function showSignInRequired() {
    render({access:'Lütfen giriş yapın.', action:'Koschei modüllerini kullanmak için giriş yapın.'}, 'Giriş yapmanız gerekiyor.');
    $('result')?.insertAdjacentHTML('beforeend','<div class="premium-actions"><a class="btn btn-primary" href="/login">Giriş Yap</a></div>');
  }

  function shell(key) {
    const config = configs[key];
    if (!config) {
      document.body.innerHTML = `${nav()}<main class="page"><section class="card"><h1>Modül bulunamadı</h1><p class="premium-note">Bu modül yapılandırılmamış.</p></section></main>`;
      return false;
    }
    document.documentElement.lang = 'tr';
    document.title = `${config.title} — Koschei Web3 Hub`;
    const formFields = config.fields.length ? `<div class="form-grid">${config.fields.map(field).join('')}</div>` : '<p class="premium-note">Bu ekran üretim verisini doğrudan okur; ek giriş gerekmez.</p>';
    document.body.innerHTML = `${nav()}<main class="page"><section class="hero"><span class="badge badge-green">${config.badge.map(esc).join(' · ')}</span><h1>${esc(config.title)}</h1><p class="page-sub">${esc(config.desc)}</p><p class="premium-note">${esc(positioning)}</p></section><form id="moduleForm" class="card stack"><h2>${esc(config.inputLabel)}</h2>${formFields}<div class="premium-actions"><button class="btn btn-primary" id="submit" type="submit">${esc(config.button)}</button><a class="btn btn-ghost" href="/pricing">Paketleri Gör</a></div><p class="premium-note">${esc(safe)}</p></form><section id="result" class="card" style="margin-top:20px"><h2>Analize hazır</h2><p class="premium-note">Analize başlamak için herkese açık veriyi girin. Ürün çıktısı yalnız backend gerçek ve kanıt temelli sonuç döndürdüğünde oluşturulur.</p></section><section class="card" style="margin-top:20px"><h2>Önerilen sonraki kontroller</h2><div class="premium-actions">${(config.next || [['Paneli Aç','/dashboard'],['Risk Analizi Başlat','/risk']]).map(([label,href])=>`<a class="btn btn-ghost" href="${esc(href)}">${esc(label)}</a>`).join('')}</div></section></main>`;
    $('moduleForm').onsubmit = event => { event.preventDefault(); run(key); };
    return true;
  }

  async function run(key) {
    const config = configs[key];
    const submit = $('submit');
    if (!config || !submit) return;
    submit.disabled = true;
    submit.innerHTML = '<span class="spinner"></span> Çalışıyor…';
    try {
      const values = formValues($('moduleForm'));
      const options = {method:config.method, headers:{'Content-Type':'application/json'}};
      if (config.method !== 'GET') options.body = JSON.stringify(config.build ? config.build(values) : values);
      const response = await KoscheiAuth.apiCall(config.endpoint, options);
      if (!response) throw new Error('Yanıt alınamadı.');
      const data = await response.json().catch(()=>({}));
      if (!response.ok) {
        if (response.status === 401 && !KoscheiAuth.getJwt()) { location.href = '/login'; return; }
        if ([401,402,403].includes(response.status) || data.code === 'PACKAGE_REQUIRED' || data.error === 'insufficient_outputs') { showPackageRequired(); return; }
        render({message:data.message || data.error || 'Analiz tamamlanamadı.', action:'Herkese açık girdiyi kontrol edip tekrar deneyin.'}, 'Analiz tamamlanamadı.');
        return;
      }
      const normalized = config.normalize ? config.normalize(data) : normalizeResult(key, data);
      render(normalized, key === 'chains' ? 'Zincir sağlık kayıtları güncellendi.' : 'Kanıt temelli analiz tamamlandı.');
    } catch (error) {
      render({message:'Analiz tamamlanamadı.', detail:error?.message || 'Bilinmeyen hata', action:'Daha sonra tekrar deneyin veya herkese açık girdiyi doğrulayın.'}, 'Analiz tamamlanamadı.');
    } finally {
      submit.disabled = false;
      submit.textContent = config.button;
    }
  }

  async function init(key) {
    if (!shell(key)) return;
    await KoscheiAuth.init();
    if (!KoscheiAuth.isLoggedIn()) { showSignInRequired(); return; }
    if (window.KoscheiAnalytics) KoscheiAnalytics.track(`${key}_module_view`);
    if (key === 'chains') run(key);
  }

  return {init};
})();