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

  function ensurePricingEmailField() {
    try {
      if (!/\/pricing/.test(window.location.pathname)) return;
      const form = document.getElementById('paymentForm');
      if (!form || form.querySelector('[name="customer_email"]')) return;
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
      form.addEventListener('submit', function() {
        const field = form.querySelector('[name="customer_email"]');
        if (field && !field.value) field.value = authEmail() || '';
      }, true);
    } catch {}
  }

  if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', ensurePricingEmailField);
  } else {
    ensurePricingEmailField();
  }
  setTimeout(ensurePricingEmailField, 400);

  return { track, ensurePricingEmailField };
}());
