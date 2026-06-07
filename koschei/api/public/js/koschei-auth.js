(function () {
  const KEY = 'koschei_jwt';
  const state = { neonAuthUrl: '' };

  function saveJwt(t) { try { localStorage.setItem(KEY, t); } catch {} }
  function getJwt() { try { return localStorage.getItem(KEY) || ''; } catch { return ''; } }
  function clearJwt() { try { localStorage.removeItem(KEY); } catch {} }

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

  function normalizeBaseUrl(raw) {
    return String(raw || '').trim().replace(/\/+$/, '');
  }

  function defaultUserName(email) {
    const name = String(email || '').split('@')[0].trim();
    return name || 'User';
  }

  function errorMessage(data, fallback) {
    if (!data) return fallback;
    if (typeof data === 'string') return data || fallback;
    return data.message || data.error_description || data.error || data.detail || fallback;
  }

  function findJwt(value) {
    if (!value || typeof value !== 'object') return '';
    const candidates = [
      value.access_token,
      value.token,
      value.session && value.session.access_token,
      value.data && value.data.session && value.data.session.access_token,
      value.data && value.data.access_token,
      value.data && value.data.token,
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

  async function loadConfig() {
    const res = await fetch('/api/config', { credentials: 'same-origin' });
    const data = await readJSON(res);
    if (res.ok) state.neonAuthUrl = normalizeBaseUrl(data.neonAuthUrl);
    return data;
  }

  function requireNeonAuthUrl() {
    if (!state.neonAuthUrl) {
      throw new Error('Neon Auth public URL is not configured. Set EXPO_PUBLIC_NEON_AUTH_URL or NEON_AUTH_BASE_URL.');
    }
    return state.neonAuthUrl;
  }

  async function neonRequest(path, body) {
    const res = await fetch(requireNeonAuthUrl() + path, {
      method: 'POST',
      credentials: 'include',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(body),
    });
    const data = await readJSON(res);
    if (!res.ok) {
      throw new Error(errorMessage(data, `Neon Auth error ${res.status}`));
    }
    return { data, headerJwt: res.headers.get('set-auth-jwt') || '' };
  }

  async function tokenFollowUp() {
    const base = requireNeonAuthUrl();
    for (const path of ['/token', '/get-session']) {
      for (const method of ['GET', 'POST']) {
        const res = await fetch(base + path, {
          method,
          credentials: 'include',
          headers: method === 'POST' ? { 'Content-Type': 'application/json' } : {},
          body: method === 'POST' ? '{}' : undefined,
        }).catch(() => null);
        if (!res) continue;
        const data = await readJSON(res);
        const headerJwt = res.headers.get('set-auth-jwt') || '';
        const jwt = _isJwt(headerJwt) ? headerJwt : findJwt(data);
        if (jwt) return jwt;
      }
    }
    return '';
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
    let jwt = _isJwt(result.headerJwt) ? result.headerJwt : findJwt(result.data);
    if (!jwt) jwt = await tokenFollowUp();
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

  async function signUp(email, password) {
    if (!state.neonAuthUrl) await loadConfig();
    const result = await neonRequest('/sign-up/email', {
      email,
      password,
      name: defaultUserName(email),
    });
    return finishAuth(result);
  }

  async function signIn(email, password) {
    if (!state.neonAuthUrl) await loadConfig();
    const result = await neonRequest('/sign-in/email', { email, password });
    return finishAuth(result);
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

// Neon Auth email/password responses are verified through /api/me and persisted as koschei_jwt.
