(function () {
  const auth = window.KoscheiAuth;
  if (!auth || window.__koscheiAuthSessionFixInstalled) return;
  window.__koscheiAuthSessionFixInstalled = true;

  const SIGNED_OUT_KEY = 'koschei_explicitly_signed_out';
  const JWT_KEYS = ['koschei_jwt', 'koschei_token'];
  const originalInit = auth.init.bind(auth);
  const originalSignIn = auth.signIn.bind(auth);
  const originalSignUp = auth.signUp.bind(auth);

  function clearLocalSession() {
    try {
      for (const key of JWT_KEYS) localStorage.removeItem(key);
    } catch {}
  }

  function markSignedOut() {
    try { localStorage.setItem(SIGNED_OUT_KEY, '1'); } catch {}
  }

  function clearSignedOut() {
    try { localStorage.removeItem(SIGNED_OUT_KEY); } catch {}
  }

  function explicitlySignedOut() {
    try { return localStorage.getItem(SIGNED_OUT_KEY) === '1'; } catch { return false; }
  }

  async function endNeonSession() {
    try {
      const configResponse = await fetch('/api/config', { credentials: 'same-origin' });
      const config = await configResponse.json().catch(() => ({}));
      const baseURL = String(config.neonAuthUrl || '').trim().replace(/\/+$/, '');
      if (!configResponse.ok || !baseURL) return;
      await fetch(baseURL + '/sign-out', {
        method: 'POST',
        credentials: 'include',
        headers: { 'Content-Type': 'application/json' },
        body: '{}',
      });
    } catch {}
  }

  auth.init = async function () {
    if (explicitlySignedOut()) {
      clearLocalSession();
      return false;
    }
    return originalInit();
  };

  auth.signIn = async function (email, password) {
    clearSignedOut();
    return originalSignIn(email, password);
  };

  auth.signUp = async function (email, password) {
    clearSignedOut();
    return originalSignUp(email, password);
  };

  auth.signOut = async function () {
    markSignedOut();
    clearLocalSession();
    await endNeonSession();
    window.location.replace('/login.html?signed_out=1');
  };
})();
