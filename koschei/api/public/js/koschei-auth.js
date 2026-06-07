(function () {
  const KEY = 'koschei_jwt';
  let _neonAuthUrl = '';

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

  async function _neonRequest(path, body) {
    if (!_neonAuthUrl) throw new Error('Auth yapılandırılmamış. Sayfayı yenileyin.');
    const res = await fetch(_neonAuthUrl + path, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(body),
    });
    const data = await res.json().catch(() => ({}));
    if (!res.ok) {
      throw new Error(data?.message || data?.error || JSON.stringify(data) || `Auth error ${res.status}`);
    }
    return data;
  }

  async function _provision() {
    const jwt = getJwt();
    if (!jwt) return;
    try {
      await fetch('/api/auth/provision', {
        method: 'POST',
        headers: { 'Authorization': 'Bearer ' + jwt },
      });
    } catch {}
  }

  async function init() {
    try {
      const res = await fetch('/api/config');
      const data = await res.json().catch(() => ({}));
      _neonAuthUrl = (data?.neonAuthUrl || '').replace(/\/$/, '');
    } catch {}
  }

  async function signUp(email, password) {
    const data = await _neonRequest('/sign-up/email', {
      email,
      password,
      name: email.split('@')[0] || 'User',
      callbackURL: 'https://tradepigloball.co/hub.html',
    });
    const jwt = data?.token || data?.access_token
      || data?.data?.token || data?.data?.access_token;
    if (!_isJwt(jwt)) throw new Error('Kayıt başarılı fakat token alınamadı.');
    saveJwt(jwt);
    await _provision();
    return data;
  }

  async function signIn(email, password) {
    const data = await _neonRequest('/sign-in/email', {
      email,
      password,
      callbackURL: 'https://tradepigloball.co/hub.html',
    });
    const jwt = data?.token || data?.access_token
      || data?.data?.token || data?.data?.access_token;
    if (!_isJwt(jwt)) throw new Error('Giriş başarılı fakat token alınamadı.');
    saveJwt(jwt);
    await _provision();
    return data;
  }

  async function signOut() {
    clearJwt();
    try { await _neonRequest('/sign-out', {}); } catch {}
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

  window.KoscheiAuth = { init, signIn, signUp, signOut,
    isLoggedIn, requireAuth, apiCall, getEmail, getSub, getJwt };
})();
