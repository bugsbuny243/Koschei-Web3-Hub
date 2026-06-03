var KoscheiAnalytics = (function() {
  function currentEmail(explicitEmail) {
    if (explicitEmail) return explicitEmail;
    try {
      if (window.KoscheiAuth && typeof window.KoscheiAuth.getEmail === 'function') {
        return window.KoscheiAuth.getEmail() || null;
      }
    } catch {}
    return null;
  }

  function payload(eventName, options = {}) {
    return {
      event_name: eventName,
      email: currentEmail(options.email),
      path: options.path || `${window.location.pathname}${window.location.search}`,
      referrer: document.referrer || '',
      user_agent: navigator.userAgent || '',
      metadata: options.metadata || {},
    };
  }

  function track(eventName, options = {}) {
    try {
      const body = JSON.stringify(payload(eventName, options));
      if (navigator.sendBeacon) {
        const sent = navigator.sendBeacon('/api/analytics/event', new Blob([body], { type: 'application/json' }));
        if (sent) return;
      }
      fetch('/api/analytics/event', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body,
        keepalive: true,
      }).catch(() => {});
    } catch {}
  }

  return { track };
}());
