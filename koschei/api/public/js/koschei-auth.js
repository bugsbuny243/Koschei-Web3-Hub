function _b64url(str) {
  str = str.replace(/-/g, '+').replace(/_/g, '/');
  while (str.length % 4) str += '=';
  try { return atob(str); } catch { return '{}'; }
}

var KoscheiAuth = (function() {
  let _neonAuthUrl = '';

  async function init() {
    try {
      const res = await fetch('/api/config');
      const cfg = await res.json();
      _neonAuthUrl = (cfg.neonAuthUrl || '').replace(/\/+$/, '');
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

  async function _neonRequest(path, body) {
    if (!_neonAuthUrl) throw new Error('Auth yapılandırılmamış. Sayfayı yenileyin.');
    const res = await fetch(`${_neonAuthUrl}${path}`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(body),
    });
    const data = await res.json().catch(() => ({}));
    if (!res.ok) {
      throw new Error(data?.message || data?.error?.message || data?.error || `Hata (${res.status})`);
    }
    return data;
  }

  async function _backendLogin(email, password) {
    const res = await fetch('/api/auth/login', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ email, password }),
    });

    const data = await res.json().catch(() => ({}));

    if (!res.ok) {
      throw new Error(data?.message || data?.error || `Giriş başarısız (${res.status})`);
    }

    const jwt = data?.access_token || data?.token || data?.data?.access_token;
    if (!_isJwt(jwt)) {
      throw new Error('Login succeeded but no auth token was returned.');
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
    const data = await _neonRequest('/sign-in/email', {
      email,
      password,
    });
    const jwt = data?.token || data?.access_token
      || data?.data?.token || data?.data?.access_token;
    if (!_isJwt(jwt)) {
      throw new Error('Giriş başarılı fakat token alınamadı.');
    }
    saveJwt(jwt);
    if (!isLoggedIn()) {
      throw new Error('Token kaydedilemedi.');
    }
    await _provision();
    return data;
  }

  async function signUp(email, password) {
    await _neonRequest('/sign-up/email', {
      email,
      password,
      name: email.split('@')[0] || 'User',
      callbackURL: '/hub'
    });
    return signIn(email, password);
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
