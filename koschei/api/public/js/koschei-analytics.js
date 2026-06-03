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

  return { track };
}());
