function _b64url(str) {
  str = str.replace(/-/g, '+').replace(/_/g, '/');
  while (str.length % 4) str += '=';
  try { return atob(str); } catch { return '{}'; }
}

var KoscheiAuth = (function() {
  async function init() {
    try {
      await fetch('/api/config');
    } catch { console.error('Koschei: config yüklenemedi'); }
  }

  function _isJwt(val) {
    if (typeof val !== 'string') return false;
    const p = val.split('.');
    return p.length === 3 && p.every(Boolean);
  }

  function saveJwt(jwt) { localStorage.setItem('koschei_jwt', jwt); }
  function getJwt() { return localStorage.getItem('koschei_jwt') || null; }

  function isLoggedIn() {
    const jwt = getJwt();
    if (!jwt) return false;
    try {
      const payload = JSON.parse(_b64url(jwt.split('.')[1]));
      return payload.exp > Math.floor(Date.now() / 1000);
    } catch { return false; }
  }

  function requireAuth(redirectTo = '/login') {
    if (!isLoggedIn()) { window.location.replace(redirectTo); return false; }
    return true;
  }

  function requireGuest(redirectTo = '/hub') {
    if (isLoggedIn()) { window.location.replace(redirectTo); return false; }
    return true;
  }

  function signOut() {
    localStorage.removeItem('koschei_jwt');
    window.location.replace('/login');
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

  async function _backendAuth(path, email, password) {
    const res = await fetch(path, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ email, password, callbackURL: 'https://tradepigloball.co/hub.html' }),
    });

    const data = await res.json().catch(() => ({}));

    if (!res.ok) {
      throw new Error(data?.message || data?.error || `Giriş başarısız (${res.status})`);
    }

    const jwt = data?.access_token || data?.token || data?.data?.access_token;
    if (!_isJwt(jwt)) {
      throw new Error('Auth succeeded but no auth token was returned.');
    }

    saveJwt(jwt);
    if (!isLoggedIn()) {
      throw new Error('Auth token could not be saved.');
    }
    return data;
  }

  async function _provision() {
    const jwt = getJwt();
    if (!jwt) return;
    try {
      await fetch('/api/auth/provision', {
        method: 'POST',
        headers: { 'Authorization': `Bearer ${jwt}` },
      });
    } catch {}
  }

  async function signIn(email, password) {
    const data = await _backendAuth('/api/auth/login', email, password);
    await _provision();
    return data;
  }

  async function signUp(email, password) {
    const data = await _backendAuth('/api/auth/register', email, password);
    await _provision();
    return data;
  }

  async function apiCall(path, options = {}) {
    const jwt = getJwt();
    const headers = { ...(options.headers || {}) };
    if (jwt) headers['Authorization'] = `Bearer ${jwt}`;
    try {
      const res = await fetch(path, { ...options, headers });
      return res;
    } catch { return null; }
  }

  return { init, signIn, signUp, signOut, isLoggedIn, requireAuth,
           requireGuest, getEmail, getSub, apiCall, getJwt };
}());
