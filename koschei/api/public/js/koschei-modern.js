// ORPHAN: no HTML page loads this file as of 2026-09-14.
(() => {
  'use strict';
  const $ = (sel, root = document) => root.querySelector(sel);
  const $$ = (sel, root = document) => Array.from(root.querySelectorAll(sel));
  const state = {
    csrf: sessionStorage.getItem('koschei_csrf') || cryptoRandom(),
    alerts: []
  };
  sessionStorage.setItem('koschei_csrf', state.csrf);

  const fmtUSD = new Intl.NumberFormat('en-US', { style: 'currency', currency: 'USD', maximumFractionDigits: 0 });
  const toast = $('#toast');

  function cryptoRandom() {
    const bytes = new Uint8Array(16);
    window.crypto.getRandomValues(bytes);
    return Array.from(bytes, (b) => b.toString(16).padStart(2, '0')).join('');
  }

  function showToast(message, type = 'ok') {
    if (!toast) return;
    toast.textContent = message;
    toast.className = `toast show ${type === 'error' ? 'error' : ''}`;
    window.setTimeout(() => toast.className = 'toast', 3600);
  }

  function setLoading(node, label = 'Yükleniyor…') {
    node.replaceChildren();
    const box = document.createElement('div');
    box.className = 'empty-state';
    const spinner = document.createElement('span');
    spinner.className = 'loader';
    const text = document.createElement('p');
    text.textContent = label;
    box.append(spinner, text);
    node.append(box);
  }

  function renderEmpty(node, title, detail) {
    node.replaceChildren();
    const box = document.createElement('div');
    box.className = 'empty-state';
    const h = document.createElement('h3');
    h.textContent = title;
    const p = document.createElement('p');
    p.textContent = detail;
    box.append(h, p);
    node.append(box);
  }

  function renderResult(node, data, preferredKeys = []) {
    node.replaceChildren();
    const grid = document.createElement('div');
    grid.className = 'result-grid';
    const safeData = data && typeof data === 'object' ? data : {};
    const entries = Object.entries(safeData);
    const ordered = [...preferredKeys.filter((key) => key in safeData).map((key) => [key, safeData[key]], ...entries.filter(([key]) => !preferredKeys.includes(key))];
    ordered.slice(0, 10).forEach(([key, value]) => {
      const item = document.createElement('div');
      item.className = 'result-item';
      const b = document.createElement('b');
      b.textContent = key.replaceAll('_', ' ');
      const span = document.createElement('span');
      if (Array.isArray(value)) span.textContent = value.join(' • ');
      else if (value && typeof value === 'object') span.textContent = JSON.stringify(value, null, 2);
      else span.textContent = String(value ?? '—');
      item.append(b, span);
      grid.append(item);
    });
    node.append(grid);
  }

  async function apiFetch(path, options = {}) {
    const headers = new Headers(options.headers || {});
    headers.set('Accept', 'application/json');
    headers.set('X-CSRF-Token', state.csrf);
    if (options.body && !headers.has('Content-Type')) headers.set('Content-Type', 'application/json');
    if (!window.KoscheiAuth || typeof window.KoscheiAuth.apiCall !== 'function') {
      throw new Error('Müşteri oturumu gerekli. Lütfen giriş yapın.');
    }
    const res = await window.KoscheiAuth.apiCall(path, { ...options, headers });
    if (!res) throw new Error('API bağlantısı kurulamadı.');
    const text = await res.text();
    let data = {};
    try { data = text ? JSON.parse(text) : {}; } catch { data = { raw: text }; }
    if (!res.ok) {
      const message = data.error || data.message || `HTTP ${res.status}`;
      throw new Error(message);
    }
    return data;
  }

  function objectFromForm(form) {
    const data = Object.fromEntries(new FormData(form).entries());
    for (const [key, value] of Object.entries(data)) {
      if (value === '') continue;
      const input = form.elements[key];
      if (input && input.type === 'number') data[key] = Number(value);
    }
    return data;
  }

  function routeTo(id) {
    const target = id || 'home';
    $$('.view').forEach((view) => view.classList.toggle('is-active', view.id === target));
    $$('.nav-links a[data-route]').forEach((link) => link.classList.toggle('active', link.dataset.route === target));
    $('#navLinks')?.classList.remove('open');
    $('.nav-toggle')?.setAttribute('aria-expanded', 'false');
    if (target === 'impact') loadImpact();
  }

  window.addEventListener('hashchange', () => routeTo(location.hash.slice(1) || 'home'));
  $$('#navLinks [data-route], .hero-actions [data-route]').forEach((link) => link.addEventListener('click', () => routeTo(link.dataset.route)));
  $('.nav-toggle')?.addEventListener('click', (event) => {
    const open = $('#navLinks').classList.toggle('open');
    event.currentTarget.setAttribute('aria-expanded', String(open));
  });

  // Authentication is managed by the canonical KoscheiAuth customer session.
  // Never persist or accept bearer/API credentials in this dashboard.
  const settingsForm = $('#settingsForm');
  if (settingsForm) {
    settingsForm.querySelectorAll('input').forEach((input) => {
      if (input.id === 'authToken' || input.id === 'apiKey') {
        input.value = '';
        input.disabled = true;
        input.placeholder = 'Müşteri oturumu kullanılıyor';
      }
    });
    settingsForm.addEventListener('submit', (event) => {
      event.preventDefault();
      showToast('Kimlik doğrulama müşteri oturumuyla yönetiliyor.');
    });
  }

  async function boot() {
    try {
      const status = await apiFetch('/api/version');
      $('#apiStatus').textContent = `${status.app || 'koschei'} hazır`;
    } catch (err) {
      $('#apiStatus').textContent = `API uyarısı: ${err.message}`;
    }
    $('#heroTools').textContent = '6';
    renderEmpty($('#walletResult'), 'Hazır', 'Cüzdan adresi girip API çağrısını başlatın.');
    renderEmpty($('#tokenResult'), 'Hazır', 'Mint adresi girip token risk sinyallerini görüntüleyin.');
    renderEmpty($('#mevResult'), 'Hazır', 'Swap parametreleriyle sandwich riskini ölçün.');
    renderEmpty($('#daoResult'), 'Hazır', 'Treasury proposal parametreleriyle risk skoru üretin.');
    renderAlerts();
    routeTo(location.hash.slice(1) || 'home');
  }

  $('#walletForm').addEventListener('submit', async (event) => {
    event.preventDefault();
    const node = $('#walletResult');
    setLoading(node, 'Cüzdan kanıtları inceleniyor…');
    try {
      const data = await apiFetch('/api/wallet/score', { method: 'POST', body: JSON.stringify(objectFromForm(event.currentTarget)) });
      renderResult(node, data, ['status', 'has_live_evidence', 'final_verdict', 'evidence', 'findings']);
      showToast('Cüzdan inceleme sonucu hazır; kanıt durumunu kontrol edin.');
    } catch (err) { renderEmpty(node, 'Cüzdan incelemesi alınamadı', err.message); showToast(err.message, 'error'); }
  });

  $('#tokenForm').addEventListener('submit', async (event) => {
    event.preventDefault();
    const node = $('#tokenResult');
    setLoading(node, 'Token taranıyor…');
    try {
      const data = await apiFetch('/api/token/scan', { method: 'POST', body: JSON.stringify(objectFromForm(event.currentTarget)) });
      renderResult(node, data, ['score', 'risk_level', 'supply', 'largest_holder_percent', 'findings']);
      showToast('Token Scanner tamamlandı.');
    } catch (err) { renderEmpty(node, 'Token taraması alınamadı', err.message); showToast(err.message, 'error'); }
  });

  $('#mevForm').addEventListener('submit', async (event) => {
    event.preventDefault();
    const node = $('#mevResult');
    const payload = objectFromForm(event.currentTarget);
    if (!payload.tx_signature && !payload.raw_transaction) {
      renderEmpty(node, 'İşlem kanıtı gerekli', 'MEV analizi için gerçek işlem imzası veya işlem verisi girin.');
      return;
    }
    setLoading(node, 'MEV simülasyonu çalışıyor…');
    try {
      const data = await apiFetch('/api/mev/analyze', { method: 'POST', body: JSON.stringify(payload) });
      renderResult(node, data, ['risk_score', 'risk_level', 'estimated_loss_usd', 'recommended_tip_sol', 'signals']);
      $('#heroSaved').textContent = Number.isFinite(Number(data.mev_saved_usd)) && data.mev_saved_usd != null
        ? fmtUSD.format(Number(data.mev_saved_usd)) : 'Doğrulanmadı';
      showToast('MEV Shield raporu hazır.');
    } catch (err) { renderEmpty(node, 'MEV analizi alınamadı', err.message); showToast(err.message, 'error'); }
  });

  async function loadImpact() {
    const metrics = $('#impactMetrics');
    const bars = $('#impactBars');
    metrics.replaceChildren(); bars.replaceChildren();
    try {
      const data = await apiFetch('/api/public/impact');
      const cards = normalizeImpact(data);
      cards.forEach((card) => addMetric(metrics, card.label, card.value));
      cards.forEach((card) => { if (card.numeric != null) addBar(bars, card.label, Math.min(100, Number(card.numeric))); });
    } catch (err) {
      addMetric(metrics, 'Etki verisi', 'Kullanılamıyor');
    }
  }

  function normalizeImpact(data) {
    const src = data.metrics || data.impact || data;
    return [
      { label: 'MEV saved', value: src.mev_saved_usd != null || src.total_mev_saved_usd != null ? fmtUSD.format(Number(src.mev_saved_usd ?? src.total_mev_saved_usd)) : 'Doğrulanmadı', numeric: src.mev_saved_usd ?? src.total_mev_saved_usd ?? null },
      { label: 'Liquidity protected', value: src.liquidity_loss_prevented_usd != null ? fmtUSD.format(Number(src.liquidity_loss_prevented_usd)) : 'Doğrulanmadı', numeric: src.liquidity_loss_prevented_usd ?? null },
      { label: 'DAO protected', value: src.proposal_loss_prevented_usd != null ? fmtUSD.format(Number(src.proposal_loss_prevented_usd)) : 'Doğrulanmadı', numeric: src.proposal_loss_prevented_usd ?? null }
    ];
  }

  function addMetric(root, label, value) {
    const div = document.createElement('div');
    const b = document.createElement('b');
    const span = document.createElement('span');
    b.textContent = value;
    span.textContent = label;
    div.append(b, span);
    root.append(div);
  }

  function addBar(root, label, value) {
    const row = document.createElement('div');
    row.className = 'bar';
    const name = document.createElement('span');
    name.textContent = label;
    const track = document.createElement('div');
    track.className = 'bar-track';
    const fill = document.createElement('div');
    fill.className = 'bar-fill';
    fill.style.width = `${Math.max(5, Math.min(100, value))}%`;
    const pct = document.createElement('span');
    pct.textContent = `${Math.round(value)}%`;
    track.append(fill);
    row.append(name, track, pct);
    root.append(row);
  }

  $('#refreshImpact').addEventListener('click', loadImpact);

  $('#liquidityForm').addEventListener('submit', async (event) => {
    event.preventDefault();
    const payload = objectFromForm(event.currentTarget);
    try {
      const data = await apiFetch('/api/liquidity/analyze', { method: 'POST', body: JSON.stringify(payload) });
      state.alerts.unshift({ pool: payload.pool_address || payload.token_mint, severity: data.severity, score: data.risk_score, message: data.message });
      state.alerts = state.alerts.slice(0, 8);
      $('#heroAlerts').textContent = String(state.alerts.length);
      renderAlerts();
      showToast('Likidite ön değerlendirmesi oluşturuldu; canlı zincir alarmı değildir.');
    } catch (err) { showToast(err.message, 'error'); }
  });

  function renderAlerts() {
    const root = $('#liquidityAlerts');
    root.replaceChildren();
    if (!state.alerts.length) {
      const empty = document.createElement('p');
      empty.className = 'muted';
      empty.textContent = 'Henüz ön değerlendirme yok. Form sonuçları canlı zincir alarmı değildir.';
      root.append(empty);
      return;
    }
    state.alerts.forEach((alert) => {
      const item = document.createElement('div');
      item.className = 'alert-item';
      const strong = document.createElement('strong');
      const pool = document.createElement('span');
      const sev = document.createElement('span');
      pool.textContent = alert.pool || 'pool';
      sev.textContent = `${alert.severity} • ${alert.score}`;
      strong.append(pool, sev);
      const msg = document.createElement('span');
      msg.textContent = alert.message || 'İstek verilerine dayalı ön değerlendirme.';
      item.append(strong, msg);
      root.append(item);
    });
  }

  $('#daoForm').addEventListener('submit', async (event) => {
    event.preventDefault();
    const node = $('#daoResult');
    const payload = objectFromForm(event.currentTarget);
    payload.instructions = String(payload.instructions || '').split('\n').map((line) => line.trim()).filter(Boolean);
    setLoading(node, 'DAO proposal riski ölçülüyor…');
    try {
      const data = await apiFetch('/api/dao/proposal-risk', { method: 'POST', body: JSON.stringify(payload) });
      renderResult(node, data, ['risk_score', 'risk_level', 'treasury_at_risk_usd', 'proposal_loss_prevented_usd', 'message']);
      showToast('DAO Guardian raporu hazır.');
    } catch (err) { renderEmpty(node, 'DAO riski alınamadı', err.message); showToast(err.message, 'error'); }
  });

  boot();
})();
