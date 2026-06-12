const PremiumModules = (() => {
  const safe = 'Özel anahtar veya seed phrase girmeyin. Yalnızca açık cüzdan, token, işlem, proje, repository ve sosyal verileri kullanın. Koschei salt-okunur istihbarat sağlar; finansal, hukuki, yatırım veya güvenlik tavsiyesi değildir.';
  const positioning = 'Koschei işlem istihbaratı, cüzdan istihbaratı, token analizi, proje istihbaratı, risk analizi, hibe hazırlığı ve ilişki haritalamayı tek bir Solana/Web3 istihbarat akışında birleştirir.';
  const $ = id => document.getElementById(id);
  const esc = value => String(value ?? '').replace(/[&<>"']/g, char => ({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[char]));
  const technicalMessage = 'Koschei sadece herkese açık veriyi analiz eder. Özel anahtar, seed phrase veya gizli müşteri verisi girmeyin.';
  const networks = [['solana-mainnet','Solana Mainnet'], ['solana-devnet','Solana Devnet'], ['solana-testnet','Solana Testnet']];
  const targetTypes = [['wallet','Cüzdan'], ['token','Token'], ['tx','İşlem'], ['project','Proje']];
  const entityTypes = [['wallet','Cüzdan'], ['token','Token'], ['tx','İşlem'], ['project','Proje']];

  const configs = {
    tx: { title:'TX Decoder', badge:['TRANSACTION INTELLIGENCE','PACKAGE ACCESS'], desc:'Açık Solana işlem imzasını işlem amacı, ilgili programlar, risk bayrakları, kanıt notları ve önerilen sonraki kontroller olarak çözün.', endpoint:'/api/tx/decode', method:'POST', button:'İşlemi Çöz', inputLabel:'Transaction signature', fields:[['sig','Transaction signature','text','Paste a Solana transaction signature'],['network','Network','select','solana-mainnet|solana-devnet|solana-testnet']], build:f=>({signature:f.sig, network:f.network}), next:[['Cüzdan Analizini Başlat','/wallet-score'],['Risk Analizi Başlat','/risk']] },
    txpro: { title:'TX Decoder', badge:['TRANSACTION INTELLIGENCE','PACKAGE ACCESS'], desc:'Açık işlem hash’i veya Solana imzasını amaç, varlıklar, risk ipuçları, kanıt notları ve takip analizi rotalarına çözün.', endpoint:'/api/web3/tx-decode-pro', method:'POST', button:'İşlemi Çöz', inputLabel:'Transaction signature or hash', fields:[['tx_hash','Transaction hash/signature','text','0x… or Solana signature'],['chain','Chain','select','solana|ethereum|base|arbitrum|polygon|optimism'],['network','Network','text','mainnet']], build:f=>f, next:[['Cüzdan Analizini Başlat','/wallet-score'],['Risk Analizi Başlat','/risk']] },
    wallet: { title:'Wallet Score', badge:['WALLET INTELLIGENCE','PACKAGE ACCESS'], desc:'Açık Solana cüzdanını aktivite, güncellik, hata oranı ve bakiye sinyalleriyle skorlayın. Sonuçlar gösterge, kırmızı bayrak, kanıt ve önerilen sonraki analizi içerir.', endpoint:'/api/wallet/score', method:'POST', button:'Cüzdan Analizini Başlat', inputLabel:'Wallet address', fields:[['address','Wallet address','text','Public Solana wallet address'],['network','Network','select','solana-mainnet|solana-devnet|solana-testnet']], build:f=>f, next:[['Portföyü İzle','/portfolio'],['Risk Analizi Başlat','/risk'],['İstihbarat Grafını Aç','/graph']] },
    token: { title:'Token Scanner', badge:['TOKEN INTELLIGENCE','PACKAGE ACCESS'], desc:'Solana token mint adresini arz, yetki, freeze, holder yoğunluğu ve kanıt temelli token risk göstergeleri açısından analiz edin.', endpoint:'/api/token/scan', method:'POST', button:'Token Analiz Et', inputLabel:'Token mint address', fields:[['mint','Token mint address','text','Public Solana token mint'],['network','Network','select','solana-mainnet|solana-devnet|solana-testnet']], build:f=>f, next:[['Risk Analizi Başlat','/risk'],['İstihbarat Grafını Aç','/graph']] },
    portfolio: { title:'Portfolio Tracker', badge:['PORTFOLIO INTELLIGENCE','PACKAGE ACCESS'], desc:'Track public SOL balances, token-account counts and recent activity across up to ten Solana wallets, then route suspicious exposure to risk analysis.', endpoint:'/api/portfolio/track', method:'POST', button:'Portföyü İzle', inputLabel:'Wallet addresses', fields:[['addresses','Wallet addresses','textarea','One public Solana wallet address per line'],['network','Network','select','solana-mainnet|solana-devnet|solana-testnet']], build:f=>({addresses:String(f.addresses||'').split(/\n|,/).map(x=>x.trim()).filter(Boolean), network:f.network}), next:[['Cüzdan Analizini Başlat','/wallet-score'],['Risk Analizi Başlat','/risk']] },
    risk: { title:'Risk Scanner', badge:['RISK ANALYSIS','PACKAGE ACCESS'], desc:'Cüzdan, token, işlem, kontrat veya proje için kanıt temelli risk değerlendirmesi çalıştırın. Her bulgu seviye, gerekçe, kanıt, güven ve aksiyonla sunulur.', endpoint:'/api/web3/risk-v2', method:'POST', button:'Risk Analizi Başlat', inputLabel:'Target', fields:[['target','Wallet / token / transaction / project','text','Public address, mint, hash or project name'],['target_type','Target type','select','wallet|token|tx|project|contract'],['chain','Chain','text','solana'],['network','Network','text','mainnet'],['notes','Evidence notes','textarea','Optional public context: explorer links, repository, website, known incident, or observed behavior']], build:f=>f, next:[['İşlemi Çöz','/tx-decoder'],['Token Analiz Et','/token-scanner'],['İstihbarat Grafını Aç','/graph']] },
    funding: { title:'Grant Readiness + Application Assistant', badge:['GRANT INTELLIGENCE','PACKAGE ACCESS'], desc:'Problem tanımı, Solana ekosistem değeri, mimari, kilometre taşları, bütçe mantığı, public-good açısı, traction kanıtı, riskler ve final taslakla hibe başvuru materyali hazırlayın.', endpoint:'/api/web3/funding-assistant/generate', method:'POST', button:'Hibe Hazırlığı Oluştur', inputLabel:'Project details', fields:[['project_name','Project name','text','Your project name'],['ecosystem','Ecosystem','text','Solana'],['project_category','Project category','select','security|developer tool|public good|infrastructure|payments|DePIN|AI agent|consumer app|gaming|education'],['short_description','Problem and solution','textarea','Describe the real problem, your solution and current proof points'],['requested_amount','Requested budget','text','$25,000'],['milestone_count','Milestone count','select','3|4|5|6'],['target_users','Target users','textarea','Builders, grant applicants, ecosystem teams, reviewers…'],['impact','Open-source / public-goods angle','textarea','Repository, docs, public dashboards, reusable APIs, public-good commitments…'],['traction','Traction / proof checklist','textarea','Users, commits, deployments, partners, metrics, prior grants or pilots…']], build:f=>({project_name:f.project_name, ecosystem:f.ecosystem, project_category:f.project_category, short_description:f.short_description, requested_amount:f.requested_amount, milestone_count:Number(f.milestone_count || 3), notes:`Target users: ${f.target_users}\nPublic-good angle: ${f.impact}\nTraction/proof: ${f.traction}`}), next:[['Proje Radarını Aç','/project-radar'],['Paketleri Gör','/pricing']] },
    projectRadar: { title:'Project Radar', badge:['PROJECT INTELLIGENCE','PACKAGE ACCESS'], desc:'Bir projeyi açık URL’ler, GitHub, ekosistem uyumu, güvenilirlik sinyalleri, kanıt noktaları, risk faktörleri ve hazırlık boşluklarıyla hibe/yatırımcı incelemesine hazırlayın.', endpoint:'/api/web3/project-radar', method:'POST', button:'Proje Radarını Aç', inputLabel:'Project details', fields:[['project_name','Project name','text','Project name'],['website_url','Website URL','text','https://your-project.example'],['twitter_handle','X / Twitter handle','text','@project'],['github_url','GitHub URL','text','https://github.com/org/repo'],['token_mint_address','Token mint address','text','Optional public token mint'],['public_wallet_address','Public wallet address','text','Optional public treasury or deployer wallet'],['ecosystem','Ecosystem','text','Solana'],['category','Category','select','security|developer tool|public good|infrastructure|payments|DePIN|AI agent|token|NFT|gaming|consumer app|education'],['description','Project overview','textarea','Describe product, users, architecture and why it matters to Solana'],['known_traction','Traction / proof points','textarea','Users, commits, deployments, revenue, grants, pilots, ecosystem integrations'],['notes','Reviewer notes','textarea','Known risks, missing proof points, repository/license status or reviewer questions']], build:f=>({project_name:f.project_name, website_url:f.website_url, twitter_handle:f.twitter_handle, github_url:f.github_url, token_mint_address:f.token_mint_address, public_wallet_address:f.public_wallet_address, ecosystem:f.ecosystem, category:f.category, description:f.description, known_traction:f.known_traction, notes:f.notes}), next:[['Hibe Hazırlığı Oluştur','/funding-assistant'],['Risk Analizi Başlat','/risk']] },
    graph: { title:'Intelligence Graph', badge:['CENTRAL INTELLIGENCE','STUDIO ACCESS'], desc:'Gerçek watchlist veya girilen açık adres verilerinden cüzdan, token, işlem, proje, risk sinyali, sybil bağlantısı ve hibe/proje ilişkilerini merkezi graf görünümünde oluşturun.', endpoint:'/api/web3/intelligence-graph/build', method:'POST', button:'İstihbarat Grafını Oluştur', inputLabel:'Graph source', fields:[['source_id','Watchlist source ID','text','Optional existing source ID from your account'],['address','Public wallet or contract','text','Public address when no source ID is used'],['chain','Chain','text','solana'],['network','Network','text','mainnet']], build:f=>f, next:[['Cüzdan Analizini Başlat','/wallet-score'],['Sybil Kontrolünü Başlat','/sybil-check'],['Risk Analizi Başlat','/risk']] },
    sybil: { title:'Sybil Checker', badge:['SYBIL INTELLIGENCE','STUDIO ACCESS'], desc:'Cüzdan, proje veya açık konuyu desteklenen cluster göstergeleri, tekrar eden örüntüler, ortak kaynak bağlantıları, kanıt, güven ve önerilen aksiyonla değerlendirin; kanıtsız suçlama üretmez.', endpoint:'/api/web3/sybil-check', method:'POST', button:'Sybil Kontrolünü Başlat', inputLabel:'Subject', fields:[['subject','Wallet, project or public identifier','text','Public wallet, project name or public identifier'],['check_type','Check type','select','grant|bounty|community|airdrop|custom']], build:f=>f, next:[['İstihbarat Grafını Aç','/graph'],['Hibe Hazırlığı Oluştur','/funding-assistant'],['Risk Analizi Başlat','/risk']] },
    metadata: { title:'Metadata Studio', badge:['PROJECT INTELLIGENCE','PACKAGE ACCESS'], desc:'Prepare production metadata and safety review notes for project, token, NFT, grant and public-good submissions.', endpoint:'/api/metadata/generate', method:'POST', button:'Generate Metadata', inputLabel:'Project name', fields:[['asset_name','Project name','text','Project name'],['description','Description','textarea','Describe the real project and evidence'],['asset_type','Category','select','builder_tool|token|nft|grant_project|public_good|dao_tool'],['ecosystem','Chain / ecosystem','text','Solana'],['website','Website','text','https://your-project.example'],['socials','Social links','textarea','https://x.com/project\nhttps://github.com/org/repo']], build:f=>({asset_name:f.asset_name, description:f.description, asset_type:f.asset_type, traits:`ecosystem:${f.ecosystem}; website:${f.website}; socials:${f.socials}`}) }
  };

  function nav() { return '<nav class="nav"><a class="nav-logo" href="/">K Koschei</a><div class="nav-links"><a class="nav-link" href="/dashboard">Panel</a><a class="nav-link" href="/pricing">Paketler</a><a class="nav-link" href="/tx-decoder">TX Decoder</a><a class="nav-link" href="/token-scanner">Token Scanner</a><a class="nav-link" href="/wallet-score">Wallet Score</a><a class="nav-link" href="/account">Hesap</a></div></nav>'; }
  function field([name,label,type,placeholder]) { if (type === 'select') return `<label class="form-label">${esc(label)}<select class="form-input" name="${esc(name)}">${placeholder.split('|').map(x=>`<option value="${esc(x)}">${esc(x)}</option>`).join('')}</select></label>`; if (type === 'textarea') return `<label class="form-label">${esc(label)}<textarea class="form-input" name="${esc(name)}" placeholder="${esc(placeholder)}"></textarea></label>`; return `<label class="form-label">${esc(label)}<input class="form-input" name="${esc(name)}" placeholder="${esc(placeholder)}"></label>`; }
  function formValues(form) { return Object.fromEntries(new FormData(form)); }

  function inputMarkup(item) {
    const common = `name="${esc(item.name)}" id="${esc(item.id || item.name)}" class="form-input"`;
    if (item.type === 'select') {
      return `<label class="form-group ${esc(item.className || '')}"><span class="form-label">${esc(item.label)}</span><select ${common}>${item.options.map(([value, label]) => `<option value="${esc(value)}">${esc(label)}</option>`).join('')}</select></label>`;
    }
    if (item.type === 'textarea') {
      return `<label class="form-group ${esc(item.className || '')}"><span class="form-label">${esc(item.label)}</span><textarea ${common} placeholder="${esc(item.placeholder)}"></textarea></label>`;
    }
    if (key === 'txpro' || key === 'tx') return {...d, transaction_purpose:d.human_summary || d.summary || d.explanation || 'Transaction decoded from available public data.', involved_wallets_or_programs:d.programs || [d.from, d.to].filter(Boolean), risk_flags:d.risk_hints || (d.risk_reason ? [d.risk_reason] : []), evidence_notes:['Provider response and decoded transaction fields from the backend endpoint.'], suggested_next_checks:['Score involved wallets', 'Risk Analizi Başlat', 'Open related entities in Intelligence Graph']};
    if (key === 'wallet') return {...d, score_breakdown:{activity:d.tx_count || d.TxCount || 'available in response', trust_level:d.level || d.label, active_days:d.active_days, balance:d.balance_sol}, red_flags:(d.score < 40 ? ['Low score based on available activity signals'] : []), evidence:['Public Solana RPC account, balance and signature history signals.'], suggested_next_analysis:['Track portfolio exposure', 'Risk Analizi Başlat', 'İstihbarat Grafını Aç']};
    if (key === 'token') return {...d, metadata_security_checks:{mint_authority:d.mint_authority || 'disabled or unavailable', freeze_authority:d.freeze_authority || 'disabled or unavailable', supply:d.supply, decimals:d.decimals}, risk_flags:d.findings || [], authority_notes:['Authority fields are read from public Solana token account data when available.'], suggested_next_analysis:['Risk Analizi Başlat', 'İstihbarat Grafını Aç']};
    if (key === 'portfolio') return {...d, risk_exposure:'Review low-liquidity or unknown token accounts manually; endpoint reports public balance and token-account counts.', suspicious_assets:'No suspicious asset classification is claimed by this endpoint.', recommended_checks:['Run Wallet Score on each wallet', 'Risk Analizi Başlat on high-exposure wallets or tokens']};
    if (key === 'funding') return {...d, sections:['problem statement','Solana ecosystem value','technical architecture','milestones','budget logic','open-source/public-goods angle','traction/proof checklist','risks and mitigation','final application draft']};
    if (key === 'graph') return {...d, graph_scope:['wallets','tokens','transactions','projects','risk signals','sybil links','grant/project relationships'], empty_state:((d.nodes||[]).length===0 && (d.edges||[]).length===0) ? 'No graph relationships found yet. Create graph data from real module analysis, watchlist sources, or submitted public addresses.' : undefined};
    if (key === 'sybil') return {...d, cluster_indicators:d.signals || [], evidence:d.signals || ['Submitted public subject checked against available account data.'], confidence:d.preliminary ? 'medium' : 'high', recommended_action:d.recommendation || 'Manual review recommended before any action.', limitation:'No sybil accusation is made without evidence; use this as a review queue signal.'};
    if (key === 'projectRadar') return {...d, grant_investor_readiness:d.opportunity_score || d.public_good_score || 'Review readiness output and missing proof points.', suggested_improvements:d.what_to_check_next || d.manual_review_notes || []};
    return d;
  }

  function render(data, status='Analiz tamamlandı.') {
    const result = $('result');
    if (!result) return;
    result.innerHTML = `<div class="result-head"><h2>${esc(status)}</h2><button class="btn btn-ghost" id="copyResult">JSON Kopyala</button></div><div class="result-grid">${Object.entries(data || {}).filter(([k])=>!['ok','raw','decoded'].includes(k)).map(([k,v])=>`<div class="result-item"><b>${esc(nice(k))}</b><span>${esc(typeof v === 'object' ? JSON.stringify(v, null, 2) : v)}</span></div>`).join('')}</div><details class="raw-json"><summary>Kanıt JSON</summary><pre>${esc(JSON.stringify(data || {}, null, 2))}</pre></details>`;
    const btn = $('copyResult');
    if (btn) btn.onclick = () => navigator.clipboard && navigator.clipboard.writeText(JSON.stringify(data || {}, null, 2));
  }

  function showPackageRequired() {
    render({access:'Aktif Koschei paketi gerekli.', action:'Devam etmek için Starter, Builder veya Studio seçin.'}, 'Aktif Koschei paketi gerekli.');
    const result = $('result');
    if (result) result.insertAdjacentHTML('beforeend','<div class="premium-actions"><a class="btn btn-primary" href="/pricing.html">Paketleri Gör</a><a class="btn btn-ghost" href="/account">Hesabı Kontrol Et</a></div>');
  }

  function showSignInRequired() {
    render({access:'Lütfen giriş yapın.', action:'Paket korumalı Koschei modüllerini kullanmak için giriş yapın.'}, 'Giriş yapmanız gerekiyor.');
    const result = $('result');
    if (result) result.insertAdjacentHTML('beforeend','<div class="premium-actions"><a class="btn btn-primary" href="/login.html">Giriş Yap</a></div>');
  }

  function shell(key) {
    const c = configs[key];
    document.title = `${c.title} — Koschei Web3 Hub`;
    document.body.innerHTML = `${nav()}<main class="page"><section class="hero"><span class="badge badge-green">${c.badge.map(esc).join(' · ')}</span><h1>${esc(c.title)}</h1><p class="page-sub">${esc(c.desc)}</p><p class="premium-note">${esc(positioning)}</p></section><form id="moduleForm" class="card stack"><h2>${esc(c.inputLabel)}</h2><div class="form-grid">${c.fields.map(field).join('')}</div><div class="premium-actions"><button class="btn btn-primary" id="submit" type="submit">${esc(c.button)}</button><a class="btn btn-ghost" href="/pricing">Paketleri Gör</a></div><p class="premium-note">${esc(safe)}</p></form><section id="result" class="card" style="margin-top:20px"><h2>Analize hazır</h2><p class="premium-note">Analize başlamak için açık veriyi girin. Backend paket erişimini doğrulayıp kanıt temelli sonuç döndürmeden ürün çıktısı oluşturulmaz.</p></section><section class="card" style="margin-top:20px"><h2>Önerilen sonraki kontroller</h2><div class="premium-actions">${(c.next || [['Paneli Aç','/dashboard'],['Risk Analizi Başlat','/risk']]).map(([label,href])=>`<a class="btn btn-ghost" href="${esc(href)}">${esc(label)}</a>`).join('')}</div></section></main>`;
    $('moduleForm').onsubmit = e => { e.preventDefault(); run(key); };
  }

  async function run(key) {
    const config = configs[key];
    const submit = $('submit');
    submit.disabled = true;
    submit.innerHTML = '<span class="spinner"></span> Çalışıyor…';
    try {
      const vals = formValues($('moduleForm'));
      const res = await KoscheiAuth.apiCall(c.endpoint, {method:c.method, headers:{'Content-Type':'application/json'}, body:JSON.stringify(c.build(vals))});
      if (!res) throw new Error('Analiz tamamlanamadı. Lütfen girdiyi kontrol edip tekrar deneyin.');
      const data = await res.json().catch(()=>({}));
      if (!res.ok) {
        if (res.status === 401 && !KoscheiAuth.getJwt()) { location.href = '/login.html'; return; }
        if (res.status === 401) return showPackageRequired();
        if (res.status === 402 || data.error === 'insufficient_outputs') return showPackageRequired();
        render({message:'Analiz tamamlanamadı. Lütfen girdiyi kontrol edip tekrar deneyin.', action:'Açık veriyi kontrol edip tekrar deneyin.'}, 'Analiz tamamlanamadı.');
        return;
      }
      render(normalizeResult(key, data, vals), 'Kanıt temelli analiz tamamlandı.');
    } catch (e) {
      render({message:'Analiz tamamlanamadı. Lütfen girdiyi kontrol edip tekrar deneyin.', action:'Daha sonra tekrar deneyin veya açık girdiyi doğrulayın.'}, 'Analiz tamamlanamadı.');
    } finally {
      submit.disabled = false;
      submit.textContent = config.button;
    }
  }

  async function init(key) {
    shell(key);
    await KoscheiAuth.init();
    if (!KoscheiAuth.isLoggedIn()) showSignInRequired();
    if (window.KoscheiAnalytics) KoscheiAnalytics.track(`${key}_module_view`);
  }

  return {init};
})();
