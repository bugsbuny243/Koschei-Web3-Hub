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
    let message = form.querySelector('[data-shopier-form-message="true"]');
    if (message) return message;
    message = document.createElement('div');
    message.setAttribute('data-shopier-form-message', 'true');
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

  function selectedProductFromModal(form) {
    const title = document.getElementById('modalTitle');
    const text = (title && title.textContent || '').toLowerCase();
    if (text.includes('professional')) return 'professional';
    if (text.includes('enterprise')) return 'enterprise';
    return 'starter';
  }

  async function submitShopierPayment(form) {
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
    if (!String(body.full_name || '').trim()) {
      setFormMessage(form, 'Ad soyad alanı gerekli.', true);
      return;
    }
    if (!String(body.payment_reference || '').trim()) {
      setFormMessage(form, 'Shopier sipariş / ödeme no gerekli.', true);
      return;
    }
    if (button) {
      button.disabled = true;
      button.textContent = 'Gönderiliyor…';
    }
    setFormMessage(form, 'Ödeme bildirimi owner paneline gönderiliyor…', false);
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
        setFormMessage(form, data.message || data.error || 'Ödeme bildirimi gönderilemedi.', true);
        return;
      }
      setFormMessage(form, data.message || 'Ödeme bildirimi owner paneline gönderildi.', false);
      track('shopier_payment_report_sent', { product_id: body.product_id });
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
      if (!form.__koscheiShopierSubmitBound) {
        form.__koscheiShopierSubmitBound = true;
        form.addEventListener('submit', function(event) {
          event.preventDefault();
          event.stopImmediatePropagation();
          const field = form.querySelector('[name="customer_email"]');
          if (field && !field.value) field.value = authEmail() || '';
          submitShopierPayment(form);
        }, true);
      }
    } catch {}
  }

  if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', ensurePricingEmailField);
  } else {
    ensurePricingEmailField();
  }
  setTimeout(ensurePricingEmailField, 400);
  setTimeout(ensurePricingEmailField, 1200);

  return { track, ensurePricingEmailField };
}());
