(function () {
  const KEY = 'koschei_jwt';
  const LEGACY_KEY = 'koschei_token';
  const state = { neonAuthUrl: '' };

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

  async function backendAuthRequest(path, body) {
    const res = await fetch(path, {
      method: 'POST',
      credentials: 'same-origin',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(body),
    });
    const data = await readJSON(res);
    if (!res.ok) {
      throw new Error(errorMessage(data, `Authentication failed (${res.status})`));
    }
    return { data, headerJwt: '' };
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
    if (!jwt && state.neonAuthUrl) jwt = await tokenFollowUp();
    if (!_isJwt(jwt)) throw new Error('Authentication succeeded, but no JWT was returned by the backend.');
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

  async function backendAuth(path, body) {
    const res = await fetch(path, {
      method: 'POST',
      credentials: 'same-origin',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(body),
    });
    const data = await readJSON(res);
    if (!res.ok) {
      throw new Error(errorMessage(data, `Authentication failed (${res.status})`));
    }
    return finishAuth({ data, headerJwt: res.headers.get('set-auth-jwt') || '' });
  }

  async function signUp(email, password) {
    return backendAuth('/api/auth/register', {
      email,
      password,
      name: defaultUserName(email),
    });
  }

  async function signIn(email, password) {
    return backendAuth('/api/auth/login', { email, password });
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

// Email/password auth is proxied through /api/auth/login and /api/auth/register, verified through /api/me, and persisted as koschei_jwt.
