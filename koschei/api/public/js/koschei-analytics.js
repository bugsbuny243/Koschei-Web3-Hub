var KoscheiAnalytics = (function() {
  function authJwt() {
    try {
      if (window.KoscheiAuth && typeof KoscheiAuth.getJwt === 'function') return KoscheiAuth.getJwt();
    } catch {}
    return null;
  }

  function authEmail() {
    try {
      if (window.KoscheiAuth && typeof KoscheiAuth.getEmail === 'function') return KoscheiAuth.getEmail();
    } catch {}
    return null;
  }

  function track(eventName, metadata = {}) {
    try {
      const headers = { 'Content-Type': 'application/json' };
      const jwt = authJwt();
      if (jwt) headers.Authorization = `Bearer ${jwt}`;
      fetch('/api/analytics/event', {
        method: 'POST',
        headers,
        keepalive: true,
        body: JSON.stringify({
          event_name: eventName,
          email: authEmail(),
          path: window.location.pathname,
          metadata: metadata || {}
        })
      }).catch(() => {});
    } catch {}
  }

  function ensureFormMessage(form) {
    let message = form.querySelector('[data-kosch-form-message="true"]') || form.querySelector('[data-shopier-form-message="true"]');
    if (message) return message;
    message = document.createElement('div');
    message.setAttribute('data-kosch-form-message', 'true');
    message.style.display = 'none';
    message.style.border = '1px solid rgba(45,238,255,.28)';
    message.style.background = 'rgba(0,255,136,.10)';
    message.style.color = '#d9ffe9';
    message.style.borderRadius = '14px';
    message.style.padding = '11px 12px';
    message.style.fontSize = '13px';
    message.style.lineHeight = '1.45';
    const actions = form.querySelector('.actions');
    form.insertBefore(message, actions || null);
    return message;
  }

  function setFormMessage(form, text, bad) {
    const message = ensureFormMessage(form);
    message.textContent = text;
    message.style.display = 'block';
    message.style.borderColor = bad ? 'rgba(255,92,124,.55)' : 'rgba(0,255,136,.45)';
    message.style.background = bad ? 'rgba(255,92,124,.14)' : 'rgba(0,255,136,.10)';
    message.style.color = bad ? '#ffd6df' : '#d9ffe9';
  }

  function normalizeProductID(value) {
    const product = String(value || '').trim().toLowerCase();
    if (product === 'builder' || product === 'pro') return 'professional';
    if (product === 'studio') return 'enterprise';
    if (product === 'professional' || product === 'enterprise' || product === 'starter') return product;
    return 'starter';
  }

  function selectedProductFromModal(form) {
    if (form && form.dataset && form.dataset.product) return normalizeProductID(form.dataset.product);
    const hidden = form && form.querySelector('[name="product_id"]');
    if (hidden && hidden.value) return normalizeProductID(hidden.value);
    const title = document.getElementById('modalTitle');
    const text = (title && title.textContent || '').toLowerCase();
    if (text.includes('professional') || text.includes('builder')) return 'professional';
    if (text.includes('enterprise') || text.includes('studio')) return 'enterprise';
    return 'starter';
  }

  async function submitKoschPayment(form) {
    const button = form.querySelector('#submitPayment') || form.querySelector('button[type="submit"]');
    const oldText = button ? button.textContent : '';
    const jwt = authJwt();
    if (!jwt) {
      setFormMessage(form, 'Giriş oturumu yok. Lütfen tekrar giriş yap.', true);
      return;
    }
    const body = Object.fromEntries(new FormData(form));
    body.product_id = selectedProductFromModal(form);
    body.registered_email = authEmail() || body.customer_email || '';
    body.customer_email = body.customer_email || body.registered_email || '';
    body.wallet_address = String(body.wallet_address || body.payer_wallet || '').trim();
    body.payer_wallet = body.wallet_address;
    body.transaction_signature = String(body.transaction_signature || body.tx_signature || body.payment_reference || '').trim();
    body.payment_reference = body.transaction_signature;
    if (!String(body.full_name || '').trim()) {
      setFormMessage(form, 'Ad soyad alanı gerekli.', true);
      return;
    }
    if (!body.wallet_address) {
      setFormMessage(form, 'Ödeme yaptığın Solana cüzdan adresi gerekli.', true);
      return;
    }
    if (!body.transaction_signature) {
      setFormMessage(form, 'KOSCH transferinin Solana transaction signature değeri gerekli.', true);
      return;
    }
    if (button) {
      button.disabled = true;
      button.textContent = 'Gönderiliyor…';
    }
    setFormMessage(form, 'KOSCH ödeme bildirimi owner paneline gönderiliyor…', false);
    try {
      const response = await fetch('/api/payments/request', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json', 'Authorization': 'Bearer ' + jwt },
        body: JSON.stringify(body)
      });
      const data = await response.json().catch(() => ({}));
      if (response.status === 401 || response.status === 403) {
        if (window.KoscheiAuth && KoscheiAuth.clearJwt) KoscheiAuth.clearJwt();
        setFormMessage(form, 'Oturum süresi doldu. Tekrar giriş yapman gerekiyor.', true);
        return;
      }
      if (!response.ok) {
        setFormMessage(form, data.message || data.error || 'KOSCH ödeme bildirimi gönderilemedi.', true);
        return;
      }
      setFormMessage(form, data.message || 'KOSCH ödeme bildirimi owner paneline gönderildi.', false);
      track('kosch_payment_report_sent', { product_id: body.product_id });
    } catch (error) {
      setFormMessage(form, 'Bağlantı hatası. Lütfen tekrar dene.', true);
    } finally {
      if (button) {
        button.disabled = false;
        button.textContent = oldText || 'Owner’a Gönder';
      }
    }
  }

  function ensurePricingEmailField() {
    try {
      if (!/\/pricing/.test(window.location.pathname)) return;
      const form = document.getElementById('paymentForm');
      if (!form) return;
      if (!form.dataset.product) form.dataset.product = 'starter';
      if (!form.querySelector('[name="customer_email"]')) {
        const grid = form.querySelector('.form-grid') || form;
        const label = document.createElement('label');
        label.className = 'form-label';
        label.textContent = 'Kayıtlı e-posta';
        const input = document.createElement('input');
        input.className = 'form-input';
        input.name = 'customer_email';
        input.type = 'email';
        input.readOnly = true;
        input.value = authEmail() || '';
        label.appendChild(input);
        grid.insertBefore(label, grid.firstChild);
      }
      ensureFormMessage(form);
      if (!form.__koscheiKoschSubmitBound) {
        form.__koscheiKoschSubmitBound = true;
        form.addEventListener('submit', function(event) {
          event.preventDefault();
          event.stopImmediatePropagation();
          const field = form.querySelector('[name="customer_email"]');
          if (field && !field.value) field.value = authEmail() || '';
          submitKoschPayment(form);
        }, true);
      }
    } catch {}
  }

  async function syncDashboardPackage() {
    try {
      if (!/\/dashboard/.test(window.location.pathname)) return;
      const jwt = authJwt();
      if (!jwt) return;
      const response = await fetch('/api/me/package', {
        method: 'GET',
        headers: { 'Authorization': 'Bearer ' + jwt, 'Content-Type': 'application/json' },
        credentials: 'include'
      });
      const data = await response.json().catch(() => ({}));
      const pack = data && (data.data || data);
      if (!pack || !pack.has_active_package) return;
      const plan = pack.plan_id || pack.plan || pack.package || 'starter';
      const remaining = pack.outputs_remaining ?? pack.remaining_outputs ?? pack.remaining ?? null;
      const total = pack.outputs_total ?? pack.total_outputs ?? null;
      const planName = document.getElementById('planName');
      const planInfo = document.getElementById('planInfo');
      const remainingOutputs = document.getElementById('remainingOutputs');
      const report = document.getElementById('report');
      const send = document.getElementById('sendBtn');
      const planBar = document.getElementById('planBar');
      if (planName) planName.textContent = String(plan).replace(/^./, c => c.toUpperCase()) + ' Plan';
      if (planInfo) planInfo.textContent = 'Paket aktif';
      if (remainingOutputs && remaining !== null && remaining !== undefined) remainingOutputs.textContent = String(remaining);
      if (send) send.disabled = false;
      if (planBar && total && remaining !== null && remaining !== undefined) {
        const pct = Math.max(4, Math.min(100, Math.round((Number(remaining) / Number(total)) * 100)));
        planBar.style.width = pct + '%';
      }
      if (report && /aktif paket gerekir/i.test(report.textContent || '')) {
        report.innerHTML = '<div class="section"><h3>Analiz hazır</h3><p>Aktif paket bulundu. Token, wallet, transaction veya proje gir; Koschei 6 modüllü risk intelligence raporu üretsin.</p></div>';
      }
    } catch {}
  }

  if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', function() {
      ensurePricingEmailField();
      syncDashboardPackage();
    });
  } else {
    ensurePricingEmailField();
    syncDashboardPackage();
  }
  setTimeout(ensurePricingEmailField, 400);
  setTimeout(ensurePricingEmailField, 1200);
  setTimeout(syncDashboardPackage, 500);
  setTimeout(syncDashboardPackage, 1500);

  return { track, ensurePricingEmailField, syncDashboardPackage };
}());