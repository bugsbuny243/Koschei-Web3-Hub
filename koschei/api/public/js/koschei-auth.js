(function () {
  const KEY = 'koschei_jwt';
  const LEGACY_KEY = 'koschei_token';
  function saveJwt(t) { try { if (t) { localStorage.setItem(KEY, t); localStorage.setItem(LEGACY_KEY, t); } } catch {} }
  function getJwt() { try { return localStorage.getItem(KEY) || localStorage.getItem(LEGACY_KEY) || ''; } catch { return ''; } }
  function clearJwt() { try { localStorage.removeItem(KEY); localStorage.removeItem(LEGACY_KEY); } catch {} }

  function _isJwt(t) {
    if (!t || typeof t !== 'string') return false;
    const p = t.split('.');
    return p.length === 3 && p.every(s => s.length > 0);
  }

  function _b64url(s) {
    s = s.replace(/-/g, '+').replace(/_/g, '/');
    while (s.length % 4) s += '=';
    try { return atob(s); } catch { return ''; }
  }

  function getEmail() {
    const jwt = getJwt();
    if (!jwt) return null;
    try { return JSON.parse(_b64url(jwt.split('.')[1])).email || null; } catch { return null; }
  }

  function getSub() {
    const jwt = getJwt();
    if (!jwt) return null;
    try { return JSON.parse(_b64url(jwt.split('.')[1])).sub || null; } catch { return null; }
  }

  function defaultUserName(email) {
    const name = String(email || '').split('@')[0].trim();
    return name || 'User';
  }

  function successCallbackURL() {
    return window.location.origin.replace(/\/+$/, '') + '/hub.html';
  }

  function errorMessage(data, fallback) {
    if (!data) return fallback;
    if (typeof data === 'string') return data || fallback;
    if (data.error === 'token_missing') return 'Account was created, but no login session was returned. Please try signing in.';
    if (data.error === 'auth_session_missing' || data.error === 'auth_verification_required') {
      return data.message || 'Account was created, but no login session was returned. Please try signing in.';
    }
    return data.message || data.error_description || data.error || data.detail || fallback;
  }

  function jwtFromHeader(value) {
    const token = String(value || '').replace(/^Bearer\s+/i, '').trim();
    return _isJwt(token) ? token : '';
  }

  function findJwt(value) {
    if (!value || typeof value !== 'object') return '';
    const candidates = [
      value.token,
      value.jwt,
      value.access_token,
      value.id_token,
      value.auth_token,
      value.data && value.data.token,
      value.data && value.data.jwt,
      value.data && value.data.access_token,
      value.data && value.data.id_token,
      value.session && value.session.token,
      value.session && value.session.jwt,
      value.session && value.session.access_token,
      value.session && value.session.id_token,
    ];
    for (const candidate of candidates) {
      if (_isJwt(candidate)) return candidate;
    }
    return '';
  }

  async function readJSON(res) {
    const text = await res.text().catch(() => '');
    if (!text) return {};
    try { return JSON.parse(text); } catch { return { message: text }; }
  }

  let configPromise;

  async function loadConfig() {
    if (!configPromise) {
      configPromise = fetch('/api/config', { credentials: 'same-origin' })
        .then(async (res) => {
          const data = await readJSON(res);
          if (!res.ok) throw new Error(errorMessage(data, 'Auth configuration is unavailable.'));
          return data;
        });
    }
    return configPromise;
  }

  async function neonAuthBaseURL() {
    const cfg = await loadConfig();
    const baseURL = String(cfg.neonAuthUrl || '').trim().replace(/\/+$/, '');
    if (!baseURL) throw new Error('Neon Auth is not configured.');
    return baseURL;
  }

  async function verifyMe(jwt) {
    const res = await fetch('/api/me', {
      method: 'GET',
      credentials: 'same-origin',
      headers: { 'Authorization': 'Bearer ' + jwt },
    });
    const data = await readJSON(res);
    if (!res.ok) {
      clearJwt();
      throw new Error(errorMessage(data, 'Token was received, but /api/me rejected it.'));
    }
    return data;
  }

  async function finishAuth(result) {
    const jwt = jwtFromHeader(result.headerJwt) || findJwt(result.data);
    if (!_isJwt(jwt)) throw new Error('Authentication succeeded, but no JWT was returned by Neon Auth.');
    saveJwt(jwt);
    const me = await verifyMe(jwt);
    return { ...result.data, me, access_token: jwt, token_type: 'Bearer' };
  }

  function consumeAccessTokenFromHash() {
    const hash = window.location.hash || '';
    if (!hash || hash.length < 2) return false;
    const params = new URLSearchParams(hash.slice(1));
    const jwt = params.get('access_token') || params.get('token') || params.get('id_token') || '';
    if (!_isJwt(jwt)) return false;
    saveJwt(jwt);
    params.delete('access_token');
    params.delete('token');
    params.delete('id_token');
    const cleanUrl = window.location.pathname + window.location.search + (params.toString() ? '#' + params.toString() : '');
    window.history.replaceState(null, document.title, cleanUrl);
    return true;
  }

  async function init() {
    consumeAccessTokenFromHash();
    try { await loadConfig(); } catch {}
  }

  async function parseNeonResponse(res) {
    const data = await readJSON(res);
    return { data, headerJwt: res.headers.get('set-auth-jwt') || res.headers.get('authorization') || '' };
  }

  async function fetchNeonJSON(baseURL, path, options = {}) {
    const res = await fetch(baseURL + path, { credentials: 'include', ...options });
    const result = await parseNeonResponse(res);
    if (!res.ok) throw new Error(errorMessage(result.data, `Neon Auth failed (${res.status})`));
    return result;
  }

  async function fetchNeonSession(baseURL) {
    const attempts = [
      ['GET', '/token'],
      ['GET', '/get-session'],
      ['POST', '/token'],
      ['POST', '/get-session'],
    ];
    for (const [method, path] of attempts) {
      try {
        const result = await fetchNeonJSON(baseURL, path, {
          method,
          headers: method === 'POST' ? { 'Content-Type': 'application/json' } : undefined,
          body: method === 'POST' ? '{}' : undefined,
        });
        if (jwtFromHeader(result.headerJwt) || findJwt(result.data)) return result;
      } catch {}
    }
    return null;
  }

  async function neonEmailAuth(path, body) {
    const baseURL = await neonAuthBaseURL();
    const result = await fetchNeonJSON(baseURL, path, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(body),
    });
    if (jwtFromHeader(result.headerJwt) || findJwt(result.data)) return finishAuth(result);
    const session = await fetchNeonSession(baseURL);
    if (session) return finishAuth(session);
    return finishAuth(result);
  }

  async function signUp(email, password) {
    return neonEmailAuth('/sign-up/email', {
      email,
      password,
      name: defaultUserName(email),
      callbackURL: successCallbackURL(),
    });
  }

  async function signIn(email, password) {
    return neonEmailAuth('/sign-in/email', { email, password, callbackURL: successCallbackURL() });
  }

  async function signOut() {
    clearJwt();
    window.location.href = '/login.html';
  }

  function isLoggedIn() { return _isJwt(getJwt()); }

  function requireAuth() {
    if (!isLoggedIn()) {
      window.location.href = '/login.html';
      return false;
    }
    return true;
  }

  async function apiCall(path, options = {}) {
    const jwt = getJwt();
    const headers = { ...(options.headers || {}) };
    if (jwt) headers['Authorization'] = 'Bearer ' + jwt;
    try {
      return await fetch(path, { ...options, headers });
    } catch { return null; }
  }

  window.KoscheiAuth = { init, signIn, signUp, signOut, consumeAccessTokenFromHash,
    isLoggedIn, requireAuth, apiCall, getEmail, getSub, getJwt };
})();

// Email/password auth calls Neon Auth directly through /api/config -> neonAuthUrl, verifies through /api/me, and persists as koschei_jwt.
