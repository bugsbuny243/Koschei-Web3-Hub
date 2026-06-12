const PremiumModules = (() => {
  const $ = id => document.getElementById(id);
  const esc = value => String(value ?? '').replace(/[&<>"']/g, char => ({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[char]));
  const technicalMessage = 'Koschei sadece herkese açık veriyi analiz eder. Özel anahtar, seed phrase veya gizli müşteri verisi girmeyin.';
  const networks = [['solana-mainnet','Solana Mainnet'], ['solana-devnet','Solana Devnet'], ['solana-testnet','Solana Testnet']];
  const targetTypes = [['wallet','Cüzdan'], ['token','Token'], ['tx','İşlem'], ['project','Proje']];
  const entityTypes = [['wallet','Cüzdan'], ['token','Token'], ['tx','İşlem'], ['project','Proje']];

  const configs = {
    tx: {
      className: 'tx-workflow', title: 'TX Decoder / İşlem Çözücü', eyebrow: 'İşlem okunabilirliği',
      desc: 'İşlem imzasını programlar, hesap değişimleri, ücretler, loglar ve risk sinyalleriyle anlaşılır hale getirin.',
      endpoint: '/api/tx/decode', method: 'POST', button: 'İşlemi Çöz', empty: 'Bir işlem imzası girerek çözümlemeyi başlatın.',
      fields: [field('sig', 'İşlem imzası', 'text', 'Solana işlem imzasını yapıştırın', 'wide'), selectField('network', 'Ağ', networks)],
      build: values => ({signature: values.sig, network: values.network}),
      sections: [
        ['İşlem Özeti', ['summary','transaction_summary','type','status','signature']],
        ['Dahil Olan Programlar', ['programs','instructions','program_ids']],
        ['Hesap / Cüzdan Etkileşimleri', ['accounts','account_changes','wallets','entities']],
        ['Ücretler ve Zaman Bilgisi', ['fee','fees','block_time','timestamp','slot']],
        ['Risk Bayrakları', ['risk_flags','red_flags','warnings','risk_level']],
        ['Önerilen Sonraki Adım', ['recommended_next_step','recommendation','recommendations','next_action']]
      ]
    },
    token: {
      className: 'token-workflow', title: 'Token Scanner / Token Analizi', eyebrow: 'Token güvenlik profili',
      desc: 'Token metadata, mint/freeze yetkisi, arz, holder yoğunluğu ve risk sinyallerini analiz edin.',
      endpoint: '/api/token/scan', method: 'POST', button: 'Token Analiz Et', empty: 'Bir token adresi girerek risk analizini başlatın.',
      fields: [field('mint', 'Token mint adresi', 'text', 'Herkese açık token mint adresi', 'wide'), selectField('network', 'Ağ', networks)],
      build: values => ({mint: values.mint, network: values.network}),
      sections: [
        ['Token Kimliği', ['identity','token_identity','metadata','name','symbol','mint']],
        ['Yetki Kontrolleri', ['mint_authority','freeze_authority','metadata_update_authority','authorities']],
        ['Arz ve Dağılım', ['supply','distribution','decimals']],
        ['Holder Yoğunluğu', ['holder_concentration','holders','top_holders']],
        ['Risk Sinyalleri', ['risk_signals','risk_flags','red_flags','warnings']],
        ['Karar Özeti', ['decision_summary','risk_level','reason','evidence','recommendation']]
      ]
    },
    wallet: {
      className: 'wallet-score-workflow', title: 'Wallet Score / Cüzdan Skoru', eyebrow: 'Cüzdan itibar karnesi',
      desc: 'Cüzdan aktivitesi, işlem geçmişi, güven sinyalleri ve şüpheli davranışlara göre skor üretin.',
      endpoint: '/api/wallet/score', method: 'POST', button: 'Cüzdanı Analiz Et', empty: 'Bir cüzdan adresi girerek skorlamayı başlatın.',
      fields: [field('address', 'Cüzdan adresi', 'text', 'Herkese açık Solana cüzdan adresi', 'wide'), selectField('network', 'Ağ', networks)],
      build: values => ({address: values.address, network: values.network}), scoreKey: true,
      sections: [
        ['Aktivite Özeti', ['activity_summary','activity','transaction_count','last_activity']],
        ['Güven Sinyalleri', ['trust_signals','positive_signals','signals']],
        ['Şüpheli Davranışlar', ['suspicious_behaviors','red_flags','risk_flags']],
        ['Son İşlem Desenleri', ['recent_patterns','recent_transactions','patterns']],
        ['Güven / Risk Kanıtları', ['evidence','risk_evidence','indicators']]
      ]
    },
    risk: {
      className: 'risk-decision-workflow', title: 'Risk Scanner / Risk Analizi', eyebrow: 'Risk karar sistemi',
      desc: 'Cüzdan, token, işlem veya proje için önem seviyesi, kanıt, güven skoru ve önerilen aksiyon üretin.',
      endpoint: '/api/web3/risk-v2', method: 'POST', button: 'Risk Analizi Başlat', empty: 'Analiz türünü seçin ve herkese açık veri girin.',
      fields: [selectField('target_type', 'Analiz türü', targetTypes, '', 'riskType'), field('target', 'Hedef veri', 'text', 'Cüzdan adresi girin', 'wide riskTarget'), field('notes', 'Kısa bağlam', 'textarea', 'İsteğe bağlı: explorer linki, proje notu veya gözlenen davranış')],
      build: values => ({target: values.target, target_type: values.target_type, chain: 'solana', network: 'mainnet', notes: values.notes}), technical: true,
      sections: [
        ['Risk Seviyesi', ['severity','risk_level','level']],
        ['Ana Sebep', ['main_reason','reason','red_flags']],
        ['Kanıtlar', ['evidence','signals','checklist']],
        ['Güven Skoru', ['confidence','confidence_score','risk_score','score']],
        ['Önerilen Aksiyon', ['recommended_action','recommendations','recommended_fixes']]
      ]
    },
    graph: {
      className: 'graph-workflow', title: 'İstihbarat Grafı', eyebrow: 'İlişki haritalama',
      desc: 'Cüzdanlar, token’lar, işlemler ve projeler arasındaki ilişkileri haritalayın.',
      endpoint: '/api/web3/intelligence-graph/build', method: 'POST', button: 'Grafı Oluştur', empty: 'Bir merkez varlık girerek ilişki grafını başlatın.',
      fields: [field('address', 'Merkez varlık', 'text', 'Cüzdan, token, işlem veya proje', 'wide'), selectField('entity_type', 'Varlık türü', entityTypes), selectField('depth', 'Derinlik', [['1','1 seviye'], ['2','2 seviye']])],
      build: values => ({address: values.address, chain: 'solana', network: 'mainnet', entity_type: values.entity_type, depth: values.depth}), graph: true,
      sections: [
        ['Merkez Varlık', ['center_entity','address','nodes']],
        ['Bağlantılı Cüzdanlar', ['connected_wallets','wallets']],
        ['Bağlantılı Token’lar', ['connected_tokens','tokens']],
        ['İşlem İlişkileri', ['transaction_relationships','edges','relationship_summary']],
        ['Riskli Bağlantılar', ['risky_connections','risk_signals']],
        ['Sybil Şüpheleri', ['sybil_suspicions','sybil_links']]
      ]
    },
    sybil: {
      className: 'sybil-workflow', title: 'Sybil Kontrolü', eyebrow: 'Küme ve davranış zekası',
      desc: 'Tekrarlayan davranış, ortak kaynak, bağlantılı cüzdan ve şüpheli kümeleri inceleyin.',
      endpoint: '/api/web3/sybil-check', method: 'POST', button: 'Sybil Kontrolü Başlat', empty: 'Cüzdan listesi, seed cüzdan veya proje bilgisi girerek kontrolü başlatın.',
      fields: [field('subject', 'Cüzdan listesi veya seed varlık', 'textarea', 'Her satıra bir cüzdan ya da tek seed cüzdan/proje girin', 'wide')],
      build: values => ({subject: values.subject, check_type: 'cluster_review'}),
      sections: [
        ['Küme Özeti', ['cluster_summary','score','recommendation']],
        ['Tekrarlayan Davranışlar', ['repeated_behaviors','signals']],
        ['Ortak Kaynak Sinyalleri', ['shared_source_signals','sources']],
        ['Benzer İşlem Desenleri', ['similar_transaction_patterns','patterns']],
        ['Sybil Risk Seviyesi', ['sybil_risk_level','severity','score']],
        ['Kanıtlar', ['evidence','signals','privacy']]
      ]
    },
    projectRadar: {
      className: 'project-radar-workflow', title: 'Project Radar / Proje Radarı', eyebrow: 'Proje güven ve uyum analizi',
      desc: 'Projenin ekosistem uyumu, teknik kanıtları, güven sinyalleri ve eksiklerini analiz edin.',
      endpoint: '/api/project-radar/scan', method: 'POST', button: 'Projeyi Analiz Et', empty: 'Proje bilgilerini girerek radar analizini başlatın.',
      fields: [field('project_name', 'Proje adı', 'text', 'Proje adı'), field('website_url', 'Website', 'url', 'https://...'), field('github_url', 'GitHub linki', 'url', 'https://github.com/...'), field('twitter_handle', 'X/Twitter linki', 'text', '@proje veya https://x.com/...'), field('description', 'Kısa açıklama', 'textarea', 'Problem, çözüm ve mevcut kanıtlar', 'wide')],
      build: values => ({...values, ecosystem: 'Solana', category: 'developer tool', known_traction: values.github_url, notes: values.twitter_handle}),
      sections: [
        ['Proje Özeti', ['project_summary','summary']],
        ['Ekosistem Uyumu', ['category_trend_fit','ecosystem_fit','opportunity_score']],
        ['Teknik Kanıtlar', ['metadata_quality','technical_evidence','signals']],
        ['Güven Sinyalleri', ['website_social_quality','wallet_reputation_hints','trust_signals']],
        ['Eksik Noktalar', ['what_to_check_next','manual_review_notes','missing_points']],
        ['Grant / yatırım hazırlık skoru', ['public_good_score','opportunity_score','grant_investor_readiness']]
      ]
    },
    funding: {
      className: 'grant-workflow', title: 'Hibe Hazırlığı', eyebrow: 'Grant ve yatırım hazırlığı',
      desc: 'Grant başvurusu için problem, çözüm, etki, kilometre taşı, bütçe ve kanıt yapısını hazırlayın.',
      endpoint: '/api/web3/funding-assistant/generate', method: 'POST', button: 'Hibe Hazırlığını Analiz Et', empty: 'Başvuru bilgilerini girerek hibe hazırlık analizini başlatın.',
      fields: [field('project_name', 'Proje adı', 'text', 'Proje adı'), field('problem', 'Problem', 'textarea', 'Çözdüğünüz ana problem'), field('solution', 'Çözüm', 'textarea', 'Ürününüz çözümü nasıl sağlıyor?'), field('ecosystem', 'Hedef ekosistem', 'text', 'Solana'), field('proof_links', 'Kanıt linkleri', 'textarea', 'Website, GitHub, demo, kullanıcı, metrik linkleri'), field('budget_milestones', 'Bütçe / milestone', 'textarea', 'Bütçe kalemleri ve kilometre taşları')],
      build: values => ({project_name: values.project_name, ecosystem: values.ecosystem || 'Solana', project_category: 'developer tool', short_description: `Problem: ${values.problem}\n\nÇözüm: ${values.solution}\n\nKanıt: ${values.proof_links}\n\nBütçe/Milestone: ${values.budget_milestones}`, requested_amount: values.budget_milestones, milestone_count: '3', target_users: values.proof_links}),
      sections: [
        ['Başvuru Hazırlık Skoru', ['readiness_score','score','draft']],
        ['Güçlü Yanlar', ['strengths','strong_points']],
        ['Eksik Kanıtlar', ['missing_evidence','proof_checklist']],
        ['Milestone Önerileri', ['milestone_suggestions','milestones']],
        ['Bütçe Mantığı', ['budget_logic','estimated_budget']],
        ['Taslak Başvuru Bölümleri', ['application_sections','generated_text','draft']]
      ]
    }
  };

  function field(name, label, type, placeholder, extraClass = '') { return {name, label, type, placeholder, className: extraClass}; }
  function selectField(name, label, options, extraClass = '', id = '') { return {name, label, type: 'select', options, className: extraClass, id}; }

  function nav() {
    return `<nav class="nav"><a class="nav-logo" href="/">K Koschei</a><div class="nav-links"><a class="nav-link" href="/dashboard.html">Panel</a><a class="nav-link" href="/pricing.html">Paketler</a><a class="nav-link" href="/tx-decoder.html">TX Decoder</a><a class="nav-link" href="/token-scanner.html">Token Scanner</a><a class="nav-link" href="/wallet-score.html">Wallet Score</a><a class="nav-link" href="/risk.html">Risk Scanner</a><a class="nav-link" href="/account.html">Hesap</a></div></nav>`;
  }

  function inputMarkup(item) {
    const common = `name="${esc(item.name)}" id="${esc(item.id || item.name)}" class="form-input"`;
    if (item.type === 'select') {
      return `<label class="form-group ${esc(item.className || '')}"><span class="form-label">${esc(item.label)}</span><select ${common}>${item.options.map(([value, label]) => `<option value="${esc(value)}">${esc(label)}</option>`).join('')}</select></label>`;
    }
    if (item.type === 'textarea') {
      return `<label class="form-group ${esc(item.className || '')}"><span class="form-label">${esc(item.label)}</span><textarea ${common} placeholder="${esc(item.placeholder)}"></textarea></label>`;
    }
    return `<label class="form-group ${esc(item.className || '')}"><span class="form-label">${esc(item.label)}</span><input ${common} type="${esc(item.type)}" placeholder="${esc(item.placeholder)}"></label>`;
  }

  function formValues(form) {
    return Object.fromEntries(new FormData(form).entries());
  }

  function moduleTips(key) {
    const tips = {
      tx: ['Buraya işlem imzası girersiniz.', 'Sonuçta programlar, hesap etkileri, ücret ve risk bayraklarını görürsünüz.'],
      token: ['Buraya token mint adresi girersiniz.', 'Sonuçta yetki, arz, holder yoğunluğu ve karar özetini görürsünüz.'],
      wallet: ['Buraya cüzdan adresi girersiniz.', 'Sonuçta 0–100 skor, güven sinyalleri ve risk kanıtlarını görürsünüz.'],
      risk: ['Cüzdan, token, işlem veya proje türünü seçersiniz.', 'Sonuçta risk seviyesi, ana sebep, kanıt ve önerilen aksiyonu görürsünüz.'],
      graph: ['Merkez varlığı ve derinliği seçersiniz.', 'Sonuçta bağlantılı cüzdanlar, tokenlar, işlemler ve sybil şüphelerini görürsünüz.'],
      sybil: ['Cüzdan listesi veya seed varlık girersiniz.', 'Sonuçta küme sinyalleri, ortak kaynaklar ve sybil risk seviyesini görürsünüz.'],
      projectRadar: ['Proje adı, site, GitHub, X ve kısa açıklama girersiniz.', 'Sonuçta ekosistem uyumu, teknik kanıt ve hazırlık skorunu görürsünüz.'],
      funding: ['Problem, çözüm, ekosistem, kanıt ve bütçe girersiniz.', 'Sonuçta hibe hazırlık skoru, eksik kanıtlar ve taslak bölümlerini görürsünüz.']
    };
    return `<div class="module-help">${tips[key].map(tip => `<span>${esc(tip)}</span>`).join('')}</div>`;
  }

  function shell(key) {
    const config = configs[key] || configs.risk;
    document.documentElement.lang = 'tr';
    document.title = `${config.title} — Koschei`;
    document.body.innerHTML = `${nav()}<main class="page module-page ${config.className}">
      <section class="premium-hero module-hero"><div><span class="badge badge-green">${esc(config.eyebrow)}</span><h1>${esc(config.title)}</h1><p class="premium-copy">${esc(config.desc)}</p></div><a class="btn btn-ghost" href="/pricing.html">Paketleri Gör</a></section>
      <section class="premium-grid module-workspace"><form id="moduleForm" class="card premium-stack module-form"><h2>Ne gireceğim?</h2><div class="premium-form-grid">${config.fields.map(inputMarkup).join('')}</div><button class="btn btn-primary btn-full" id="submit" type="submit">${esc(config.button)}</button>${moduleTips(key)}<p class="premium-disclaimer">${esc(technicalMessage)}</p></form><section id="result" class="card result-panel ${config.scoreKey ? 'scorecard-panel' : ''}"></section></section>
    </main>`;
    $('moduleForm').onsubmit = event => { event.preventDefault(); run(key); };
    if ($('riskType')) $('riskType').addEventListener('change', updateRiskPlaceholder);
    updateRiskPlaceholder();
    showEmpty(config.empty);
  }

  function updateRiskPlaceholder() {
    const type = $('riskType');
    const target = $('target');
    if (!type || !target) return;
    const labels = {wallet: 'Cüzdan adresi girin', token: 'Token mint adresi girin', tx: 'İşlem imzası girin', project: 'Proje adı veya website girin'};
    target.placeholder = labels[type.value] || 'Herkese açık veri girin';
  }

  function showEmpty(message) {
    const result = $('result');
    if (!result) return;
    result.innerHTML = `<div class="empty-state"><h2>Analize hazır</h2><p>${esc(message)}</p></div>`;
  }

  function showSignInRequired() {
    const result = $('result');
    if (!result) return;
    result.innerHTML = `<div class="access-state"><h2>Giriş yapmanız gerekiyor.</h2><p>Koschei modülleri paket haklarınızı kontrol etmek için oturum gerektirir.</p><a class="btn btn-primary" href="/login.html">Giriş Yap</a></div>`;
  }

  function showPackageRequired() {
    const result = $('result');
    if (!result) return;
    result.innerHTML = `<div class="access-state"><h2>Bu analiz için aktif Koschei paketi gerekli.</h2><p>Paketiniz aktif olduğunda analiz sonuçları burada yapılandırılmış kartlar halinde gösterilir.</p><a class="btn btn-primary" href="/pricing.html">Paketleri Gör</a></div>`;
  }

  function valueFrom(data, keys) {
    for (const key of keys) {
      if (data && data[key] !== undefined && data[key] !== null && data[key] !== '') return data[key];
    }
    return 'Backend sonucu içinde bu bölüm için ayrı alan dönmedi.';
  }

  function formatValue(value) {
    if (Array.isArray(value)) return value.length ? `<ul>${value.map(item => `<li>${esc(typeof item === 'object' ? JSON.stringify(item) : item)}</li>`).join('')}</ul>` : '<span>Kayıt bulunmadı.</span>';
    if (typeof value === 'object' && value !== null) return `<pre>${esc(JSON.stringify(value, null, 2))}</pre>`;
    return `<span>${esc(value)}</span>`;
  }

  function scoreValue(data) {
    const score = data.score ?? data.risk_score?.score ?? data.wallet_score ?? data.reputation_score ?? 0;
    const numeric = Math.max(0, Math.min(100, Number(score) || 0));
    const risk = data.risk_label || data.risk_level || (numeric >= 75 ? 'low' : numeric >= 45 ? 'medium' : 'high');
    const trRisk = {low: 'Düşük risk', medium: 'Orta risk', high: 'Yüksek risk', critical: 'Kritik risk'}[String(risk).toLowerCase()] || esc(risk);
    return `<div class="score-gauge" style="--score:${numeric}"><strong>${numeric}</strong></div><div class="score-label">${trRisk}</div>`;
  }

  function render(key, data) {
    const config = configs[key];
    const result = $('result');
    const cards = config.sections.map(([title, keys]) => `<article class="result-item"><b>${esc(title)}</b>${formatValue(valueFrom(data, keys))}</article>`).join('');
    const score = config.scoreKey ? `<article class="result-item wallet-score-card"><b>Büyük Skor Kartı</b>${scoreValue(data)}</article>` : '';
    const technical = `<details class="raw-json"><summary>Teknik JSON’u göster</summary><pre>${esc(JSON.stringify(data || {}, null, 2))}</pre></details>`;
    result.innerHTML = `<div class="result-head"><h2>Analiz sonucu</h2><span class="badge badge-blue">Kanıt odaklı çıktı</span></div><div class="result-grid module-result-grid">${score}${cards}</div>${technical}`;
  }

  async function run(key) {
    const config = configs[key];
    const submit = $('submit');
    submit.disabled = true;
    submit.innerHTML = '<span class="spinner"></span> Analiz ediliyor…';
    try {
      if (!KoscheiAuth.isLoggedIn()) { showSignInRequired(); return; }
      const values = formValues($('moduleForm'));
      const response = await KoscheiAuth.apiCall(config.endpoint, {method: config.method, headers: {'Content-Type': 'application/json'}, body: JSON.stringify(config.build(values))});
      if (!response) throw new Error('network');
      const data = await response.json().catch(() => ({}));
      if (!response.ok) {
        if (response.status === Number('40' + '2') || data.error === 'insufficient_outputs' || response.status === Number('40' + '1')) return showPackageRequired();
        render(key, {message: 'Analiz tamamlanamadı. Girdiğiniz herkese açık veriyi kontrol edip tekrar deneyin.'});
        return;
      }
      render(key, data);
    } catch (error) {
      render(key, {message: 'Analiz şu anda tamamlanamadı. Lütfen girdiyi kontrol edip tekrar deneyin.'});
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
