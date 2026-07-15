(() => {
  'use strict';

  const $ = id => document.getElementById(id);
  const esc = value => String(value ?? '').replace(/[&<>"']/g, ch => ({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[ch]));
  const num = (value, digits = 2) => {
    const n = Number(value);
    return Number.isFinite(n) ? n.toLocaleString('tr-TR', { maximumFractionDigits: digits }) : '—';
  };
  const short = value => {
    const text = String(value || '');
    return text.length > 30 ? `${text.slice(0, 13)}…${text.slice(-10)}` : (text || '—');
  };
  const relative = value => {
    const date = value ? new Date(value) : null;
    if (!date || Number.isNaN(date.getTime())) return '—';
    const minutes = Math.max(0, Math.round((Date.now() - date.getTime()) / 60000));
    if (minutes < 1) return 'şimdi';
    if (minutes < 60) return `${minutes} dk`;
    if (minutes < 1440) return `${Math.round(minutes / 60)} sa`;
    return `${Math.round(minutes / 1440)} gün`;
  };
  const riskClass = value => {
    const risk = Number(value || 0);
    return risk >= 65 ? 'bad' : risk >= 35 ? 'warn' : 'good';
  };
  const boolText = (value, known = true) => !known ? 'GÖZLENMEDİ' : value ? 'AÇIK' : 'REVOKED';
  const safeJSON = value => {
    try { return JSON.stringify(value ?? {}, null, 2); } catch { return '{}'; }
  };

  const state = { cards: new Map(), access: false, currentTarget: '' };

  async function api(path, options = {}) {
    const response = await KoscheiAuth.apiCall(path, options);
    const data = await response.json().catch(() => ({}));
    return { response, data };
  }

  function notice(message, bad = false) {
    const node = $('notice');
    node.textContent = message;
    node.className = `notice show${bad ? ' bad' : ''}`;
  }

  function clearNotice() {
    $('notice').className = 'notice';
  }

  function verified(item) {
    const signals = item?.signals || {};
    return item?.signed === true && (signals.verified_evidence === true || signals.real_onchain_evidence === true || signals.real_offchain_evidence === true);
  }

  function risky(item) {
    const signals = item?.signals || {};
    const risk = Number(item?.risk_index || 0);
    return risk >= 65 || signals.mint_authority_present === true || signals.structural_mint_authority_present === true || Number(signals.structural_top10_holder_pct || 0) >= 75;
  }

  function displayName(item) {
    const signals = item?.signals || {};
    return signals.token_symbol || signals.symbol || signals.token_name || signals.name || short(item?.target);
  }

  function renderCards(items) {
    state.cards.clear();
    const visible = (items || []).filter(verified);
    const dangerItems = visible.filter(risky);
    const monitorItems = visible.filter(item => !risky(item));
    $('visible').textContent = num(visible.length, 0);
    $('floored').textContent = num(visible.filter(item => Number(item.signals?.structural_floor || 0) > 0).length, 0);
    $('redCount').textContent = dangerItems.length;
    $('greenCount').textContent = monitorItems.length;

    const cardHTML = item => {
      const key = String(item.id || item.signature || item.target).replace(/[^a-zA-Z0-9_-]/g, '').slice(0, 70);
      state.cards.set(key, item);
      const signals = item.signals || {};
      const summary = item.summary || {};
      const creator = signals.creator_wallet || signals.deployer_wallet || signals.creator || '';
      const floor = Number(signals.structural_floor || 0);
      return `<article class="radar-card" data-key="${esc(key)}">
        <div class="cardtop"><div><div class="project">${esc(displayName(item))}</div><div class="token">${esc(item.target)}</div></div><span class="badge ${risky(item) ? 'red' : 'green'}">${risky(item) ? 'WARNING' : 'MONITOR'}</span></div>
        <div class="mini"><span>Temsilci risk</span><b>${esc(item.risk_index)}/100</b></div>
        ${floor ? `<div class="mini"><span>Yapısal taban</span><b>${esc(floor)}/100</b></div>` : ''}
        ${creator ? `<div class="mini"><span>Creator / deployer</span><b>${esc(short(creator))}</b></div>` : ''}
        <div class="mini"><span>Gözlem</span><b>${esc(summary.occurrence_count || item.occurrence_count || 1)}</b></div>
        <div class="mini"><span>Son görülen</span><b>${esc(relative(summary.last_seen_at || item.created_at))}</b></div>
      </article>`;
    };

    $('danger').innerHTML = dangerItems.length ? dangerItems.map(cardHTML).join('') : '<div class="empty">Doğrulanmış yüksek risk kartı yok.</div>';
    $('monitor').innerHTML = monitorItems.length ? monitorItems.map(cardHTML).join('') : '<div class="empty">Doğrulanmış izleme kartı yok.</div>';
    document.querySelectorAll('[data-key]').forEach(node => node.addEventListener('click', () => {
      const item = state.cards.get(node.dataset.key);
      if (item) openDetail(item.target, item);
    }));
    return visible;
  }

  function renderStatus(stream = {}) {
    const manual = stream.enabled === false || ['waiting_for_stream', 'stale'].includes(String(stream.pipeline_status || ''));
    $('statusDot').className = `dot ${manual ? '' : 'live'}`;
    $('statusText').textContent = manual ? 'Manuel koruma canlı · quota saver' : 'ARVIS işlem hattı canlı';
    $('statusNote').textContent = manual ? 'Otomatik akış duraklatılmış olabilir; kullanıcı taramaları ve tam rapor çalışır.' : 'Pump keşfi, yapısal hafıza ve imzalı karar zinciri çalışıyor.';
    $('processed').textContent = num(stream.processing_completed, 0);
    $('insufficient').textContent = num(stream.processing_insufficient, 0);
    $('lastDecision').textContent = relative(stream.last_processed_at);
  }

  async function loadAccess() {
    const { response, data } = await api('/api/auth/premium-access');
    const access = data.access || {};
    state.access = response.ok && access.active === true;
    $('accessPill').textContent = state.access ? `KOSCH ${String(access.token_tier || 'basic').toUpperCase()} · TAM RAPOR` : 'KOSCH doğrulaması gerekli';
    $('accessPill').className = `pill ${state.access ? 'green' : 'amber'}`;
  }

  async function loadFeed() {
    const { response, data } = await api('/api/v1/radar/feed');
    if (!response.ok) {
      notice(response.status === 401 ? 'Giriş yapmanız gerekiyor.' : 'Tam Radar için doğrulanmış KOSCH holder access gerekir.', true);
      return [];
    }
    renderStatus(data.stream || {});
    return renderCards(data.items || []);
  }

  function moduleName(module) {
    return module.module || module.module_id || 'ARVIS module';
  }

  function moduleCards(modules) {
    if (!Array.isArray(modules) || !modules.length) return '<div class="empty">Modül sonucu yok.</div>';
    return modules.map(module => {
      const known = module.verified === true;
      const cls = known ? riskClass(module.risk_index) : 'unknown';
      const evidence = Array.isArray(module.evidence) ? module.evidence : [];
      return `<article class="module ${cls}">
        <header><div><span class="eyebrow">${esc(module.module_id || 'module')}</span><h4>${esc(moduleName(module))}</h4></div><b>${known ? `${esc(module.risk_index)}/100` : 'NO DATA'}</b></header>
        <p>${esc(module.verdict || module.recommendation || 'Kanıt toplanamadı.')}</p>
        ${evidence.length ? `<ul>${evidence.map(item => `<li>${esc(item)}</li>`).join('')}</ul>` : ''}
        <details><summary>Tüm doğrulanmış sinyaller</summary><pre>${esc(safeJSON(module.signals || {}))}</pre></details>
      </article>`;
    }).join('');
  }

  function evidenceCards(evidence) {
    if (!Array.isArray(evidence) || !evidence.length) return '<div class="empty">Kanıt kaydı yok.</div>';
    return evidence.map(row => `<div class="evidence-row ${row.verified ? 'verified' : 'unavailable'}"><b>${esc(row.module || row.module_id)}</b><span>${esc(row.text)}</span><small>${row.verified ? 'VERIFIED' : 'INSUFFICIENT EVIDENCE'}</small></div>`).join('');
  }

  function accountRows(distribution) {
    const accounts = Array.isArray(distribution?.top_accounts) ? distribution.top_accounts : [];
    if (!accounts.length) return '<div class="empty">İlk hesap dağılımı RPC tarafından alınamadı.</div>';
    return `<div class="account-list">${accounts.map(account => `<div class="account-row"><b>#${esc(account.rank)}</b><code>${esc(account.token_account)}</code><span>${num(account.balance, 6)}</span><strong>%${num(account.percentage, 4)}</strong></div>`).join('')}</div><p class="fine">Bunlar token hesaplarıdır. Gerçek cüzdan sahibi eşlemesi ayrıca parsed owner mapping gerektirir.</p>`;
  }

  function graphRows(graph) {
    const nodes = Array.isArray(graph?.nodes) ? graph.nodes : [];
    const edges = Array.isArray(graph?.edges) ? graph.edges : [];
    if (!nodes.length) return '<div class="empty">Doğrulanmış ilişki düğümü bulunamadı.</div>';
    return `<div class="graph-list">${nodes.map(node => `<div class="graph-row"><span class="pill ${node.node_type === 'creator_wallet' ? 'amber' : ''}">${esc(node.node_type)}</span><b>${esc(node.label)}</b><code>${esc(node.address || node.node_id)}</code><strong>${esc(node.risk_level || 'unknown')}</strong></div>`).join('')}</div><p class="fine">Bağlantı sayısı: ${edges.length}. Grafik, yalnız verdict sinyallerinden üretilen adres ilişkilerini gösterir.</p>`;
  }

  function bar(label, value, badAt = 50) {
    const pct = Math.max(0, Math.min(100, Number(value || 0)));
    return `<div class="barline"><label>${esc(label)}</label><div class="track"><div class="fill ${pct >= badAt ? 'bad' : ''}" style="width:${pct}%"></div></div><b>%${num(pct, 2)}</b></div>`;
  }


  function renderVerdictCard(data) {
    if (!window.KoscheiVerdictCard) return '';
    const vm = window.KoscheiVerdictCard.mapVerdictCard(data, { lang: 'en' });
    const h = vm.header;
    const headerMain = h.state === 'gathering' ? `<div class="vc-hourglass">${esc(h.icon)}</div>` : `<strong>${esc(h.grade || '—')}</strong>`;
    const leverage = vm.leverage.length ? vm.leverage.map(row => `<a class="vc-row red" href="${esc(row.evidence_anchor)}"><span></span><b>${esc(row.text)}</b></a>`).join('') : '<div class="vc-empty">No verified owner leverage rows yet.</div>';
    return `<section class="verdict-card ${esc(h.tone)}" id="verdict-card"><div class="vc-header"><div class="vc-grade">${headerMain}</div><div><span class="eyebrow">Investor-readable verdict card</span><h2>${esc(h.title)}</h2><p>${esc(h.copy)}</p><a class="vc-meta" href="#full-report-detail">Ruleset ${esc(h.ruleset_version)} · signature ${esc(h.signature_short || '—')} · generated ${esc(h.generated_at || '—')}</a></div></div><div class="vc-block"><h3>${esc(vm.leverage_title)}</h3><div class="vc-list">${leverage}</div></div><div class="vc-block"><h3>${esc(vm.checklist_title)}</h3><div class="vc-list">${vm.checklist.map(row => `<a class="vc-row ${esc(row.status)}" id="evidence-${esc(row.id)}" href="${esc(row.evidence_anchor)}"><span></span><b>${esc(row.label)}</b><em>${esc(row.value)}</em></a>`).join('')}</div></div><p class="vc-disclaimer">${esc(vm.disclaimer)}</p></section><div id="full-report-detail"></div>`;
  }

  function renderDetail(data, fallbackItem = {}) {
    const final = data.final_verdict || {};
    const warning = data.warning || {};
    const distribution = data.holder_distribution || {};
    const structural = data.structural_memory || {};
    const source = data.source_context || {};
    const modules = Array.isArray(data.modules) ? data.modules : [];
    const graph = data.graph || {};
    const signals = final.signals || fallbackItem.signals || {};
    const risk = Number(final.risk_index ?? fallbackItem.risk_index ?? 0);
    const creator = source.creator_wallet || signals.creator_wallet || signals.deployer_wallet || signals.creator || '';
    const tokenName = source.token_name || signals.token_name || signals.name || '';
    const tokenSymbol = source.token_symbol || signals.token_symbol || signals.symbol || '';
    const authorityModule = modules.find(module => module.module_id === 'token_authority_scanner') || {};
    const authoritySignals = authorityModule.signals || {};
    const authorityKnown = authorityModule.verified === true || structural.has_authority_data === true;
    const mintAuthority = structural.has_authority_data === true ? structural.mint_authority_present : authoritySignals.mint_authority_present;
    const freezeAuthority = structural.has_authority_data === true ? structural.freeze_authority_present : authoritySignals.freeze_authority_present;
    const top1 = distribution.top_1_percentage ?? structural.largest_holder_percentage;
    const top10 = distribution.top_10_percentage ?? structural.top_10_holder_percentage;
    const reasons = Array.isArray(warning.reasons) ? warning.reasons : [];
    const positives = Array.isArray(warning.positive_signals) ? warning.positive_signals : [];

    $('reportTitle').textContent = tokenSymbol || tokenName || short(data.target || fallbackItem.target);
    $('reportBody').className = 'detail-body';
    $('reportBody').innerHTML = `
      ${renderVerdictCard({...data, final_verdict: final})}
      <section class="verdict-head ${riskClass(risk)}">
        <div class="scorebox"><strong>${esc(risk)}</strong><span>RISK / 100</span></div>
        <div><span class="eyebrow">${esc(warning.label || final.risk_level || 'ARVIS VERDICT')}</span><h2>${esc(final.verdict || fallbackItem.verdict || 'İmzalı ARVIS kararı')}</h2><div class="target-full">${esc(data.target || fallbackItem.target)}</div><p class="muted">${esc(final.recommendation || fallbackItem.recommendation || 'Tüm kanıtları inceleyin.')}</p><div class="actions"><span class="pill ${risk >= 65 ? 'red' : risk >= 35 ? 'amber' : 'green'}">${esc(final.risk_level || fallbackItem.risk_level || 'unknown')}</span><span class="pill">${esc(final.grade || fallbackItem.grade || '—')}</span><span class="pill violet">${esc(source.launch_platform || 'Solana')}</span></div></div>
      </section>

      ${creator ? `<section class="creator-warning"><span class="eyebrow">CREATOR / DEPLOYER RELATION</span><h3>${esc(source.creator_label || 'Source-reported creator/deployer wallet')}</h3><code>${esc(creator)}</code><p>${esc(source.creator_scope || 'Observed source relation. This is not proof of wrongdoing or real-world identity.')}</p></section>` : ''}

      <section class="statgrid">
        <article class="stat"><label>Token</label><strong>${esc([tokenName, tokenSymbol && `(${tokenSymbol})`].filter(Boolean).join(' ') || 'Metadata yok')}</strong><small>${esc(source.launch_platform || 'Solana')}</small></article>
        <article class="stat"><label>Toplam arz</label><strong>${distribution.available ? num(distribution.supply, 6) : 'GÖZLENMEDİ'}</strong><small>RPC token supply</small></article>
        <article class="stat"><label>En büyük hesap</label><strong>${distribution.available ? num(distribution.largest_account_balance, 6) : 'GÖZLENMEDİ'}</strong><small>Token account balance</small></article>
        <article class="stat"><label>Yapısal floor</label><strong>${signals.structural_floor ? `${esc(signals.structural_floor)}/100` : structural.available ? 'AKTİF HAFIZA' : 'YOK'}</strong><small>${esc(structural.holder_observed_at || structural.authority_observed_at || '—')}</small></article>
      </section>

      <section class="two-col">
        <article class="panel"><span class="eyebrow">HOLDER CONCENTRATION</span><h3>Arz yoğunlaşması</h3><div class="bars">${bar('TOP 1', top1, 35)}${bar('TOP 3', distribution.top_3_percentage, 50)}${bar('TOP 10', top10, 75)}${bar('TOP 20', distribution.top_20_percentage, 85)}</div><p class="fine">Top 3 ve Top 20, canlı RPC largest-accounts listesinden hesaplanır. Veri yoksa sayı uydurulmaz.</p></article>
        <article class="panel"><span class="eyebrow">AUTHORITY STATUS</span><h3>Yetki durumu</h3><div class="authority"><div class="authority-card ${authorityKnown && !mintAuthority ? 'ok' : mintAuthority ? 'bad' : ''}"><label>Mint authority</label><strong>${boolText(mintAuthority, authorityKnown)}</strong></div><div class="authority-card ${authorityKnown && !freezeAuthority ? 'ok' : freezeAuthority ? 'bad' : ''}"><label>Freeze authority</label><strong>${boolText(freezeAuthority, authorityKnown)}</strong></div></div></article>
      </section>

      <section class="two-col">
        <article class="panel"><span class="eyebrow">WARNING EXPLANATION</span><h3>Neden işaretlendi?</h3><div class="insights">${reasons.map(reason => `<div class="insight bad">${esc(reason)}</div>`).join('') || '<div class="insight">Ek risk açıklaması yok.</div>'}</div></article>
        <article class="panel"><span class="eyebrow">POSITIVE SIGNALS</span><h3>Olumlu gözlemler</h3><div class="insights">${positives.map(reason => `<div class="insight good">${esc(reason)}</div>`).join('') || '<div class="insight">Doğrulanmış olumlu authority sinyali yok veya veri alınamadı.</div>'}</div></article>
      </section>

      <section class="panel full"><span class="eyebrow">ALL ARVIS MODULES</span><h3>Tüm modül sonuçları</h3><div class="module-grid">${moduleCards(modules)}</div></section>
      <section class="panel full"><span class="eyebrow">RELATION GRAPH</span><h3>Creator, funding ve cüzdan ilişkileri</h3>${graphRows(graph)}</section>
      <section class="panel full"><span class="eyebrow">TOP TOKEN ACCOUNTS</span><h3>İlk ${esc(distribution.observed_account_count || 0)} token hesabı</h3>${accountRows(distribution)}</section>
      <section class="panel full"><span class="eyebrow">COMPLETE EVIDENCE LOG</span><h3>Bütün kanıt açıklamaları</h3><div class="evidence-list">${evidenceCards(data.evidence)}</div></section>
      <section class="panel full"><span class="eyebrow">SOURCE & FINAL SIGNALS</span><h3>Ham doğrulanmış sinyaller</h3><details open><summary>Final verdict signals</summary><pre>${esc(safeJSON(signals))}</pre></details><details><summary>Launch/source signals</summary><pre>${esc(safeJSON(source.signals || {}))}</pre></details><div class="signature">İmza: ${esc(final.signature || fallbackItem.signature || '—')}</div></section>
      <p class="disclaimer">${esc(warning.interpretation || 'Koschei kanıt işaretler; suçlama veya finansal tavsiye üretmez.')}</p>
    `;
    $('reportBody').scrollIntoView({ behavior: 'smooth', block: 'start' });
  }

  async function openDetail(target, fallbackItem = {}) {
    const clean = String(target || '').trim();
    if (!clean) return;
    state.currentTarget = clean;
    $('reportTitle').textContent = 'Tam ARVIS raporu hazırlanıyor';
    $('reportBody').className = 'empty';
    $('reportBody').textContent = 'Holder dağılımı, creator/deployer ilişkisi, authority, modül kanıtları ve grafik toplanıyor…';
    try {
      const { response, data } = await api(`/api/v1/radar/detail?target=${encodeURIComponent(clean)}&network=solana-mainnet`);
      if (!response.ok) throw new Error(data.message || data.error || 'Detay raporu alınamadı.');
      renderDetail(data, fallbackItem);
    } catch (error) {
      notice(error.message || 'Detay raporu alınamadı.', true);
      $('reportBody').className = 'empty';
      $('reportBody').textContent = 'Tam rapor alınamadı. Feed kartındaki imzalı karar korunuyor.';
    }
  }

  async function runScan() {
    const target = $('target').value.trim();
    if (!target) {
      notice('Kontrol edilecek Solana mintini girin.', true);
      return;
    }
    clearNotice();
    $('run').disabled = true;
    $('run').textContent = 'KANIT TOPLANIYOR…';
    try {
      const { response, data } = await api('/api/v1/radar/check', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ target, network: 'solana-mainnet', mode: 'manual_dashboard_check' })
      });
      if (!response.ok) throw new Error(data.message || data.error || 'Tarama tamamlanamadı.');
      const items = await loadFeed();
      const item = items.find(row => String(row.target || '').toLowerCase() === target.toLowerCase()) || data.final_verdict || {};
      await openDetail(target, item);
    } catch (error) {
      notice(error.message || 'ARVIS yanıtı kullanılamıyor.', true);
    } finally {
      $('run').disabled = false;
      $('run').textContent = 'TAM ARVIS RADARI ÇALIŞTIR';
    }
  }

  async function boot() {
    await KoscheiAuth.init();
    if (!KoscheiAuth.requireAuth('/login')) return;
    $('run').addEventListener('click', runScan);
    const initialTarget = new URLSearchParams(location.search).get('target') || '';
    if (initialTarget) $('target').value = initialTarget;
    await Promise.all([loadAccess(), loadFeed()]);
    if (initialTarget && state.access) await openDetail(initialTarget);
    window.setInterval(loadFeed, 30000);
  }

  boot();
})();
