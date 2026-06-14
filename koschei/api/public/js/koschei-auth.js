(function () {
  const KEY = 'koschei_jwt';
  const LEGACY_KEY = 'koschei_token';

  function saveJwt(token) {
    try {
      if (token) {
        localStorage.setItem(KEY, token);
        localStorage.setItem(LEGACY_KEY, token);
      }
    } catch {}
  }

  function getJwt() {
    try {
      return localStorage.getItem(KEY) || localStorage.getItem(LEGACY_KEY) || '';
    } catch {
      return '';
    }
  }

  function clearJwt() {
    try {
      localStorage.removeItem(KEY);
      localStorage.removeItem(LEGACY_KEY);
    } catch {}
  }

  function isJwt(token) {
    return typeof token === 'string' && token.split('.').length === 3;
  }

  function decodePart(value) {
    value = String(value || '').replace(/-/g, '+').replace(/_/g, '/');
    while (value.length % 4) value += '=';
    try { return atob(value); } catch { return ''; }
  }

  function jwtPayload(token) {
    try { return JSON.parse(decodePart(String(token || '').split('.')[1] || '')); } catch { return {}; }
  }

  function isJwtExpired(token) {
    const exp = Number(jwtPayload(token).exp || 0);
    return exp ? exp <= Math.floor(Date.now() / 1000) + 15 : false;
  }

  function getEmail() {
    const token = getJwt();
    return token ? (jwtPayload(token).email || null) : null;
  }

  function getSub() {
    const token = getJwt();
    return token ? (jwtPayload(token).sub || null) : null;
  }

  async function readJSON(response) {
    const text = await response.text().catch(() => '');
    if (!text) return {};
    try { return JSON.parse(text); } catch { return { message: text }; }
  }

  function findJwt(data) {
    if (!data || typeof data !== 'object') return '';
    const candidates = [data.token, data.jwt, data.access_token, data.id_token, data.data && data.data.token, data.data && data.data.jwt, data.data && data.data.access_token];
    for (const item of candidates) {
      if (isJwt(item)) return item;
    }
    return '';
  }

  function message(data, fallback) {
    const raw = String((data && (data.message || data.error_description || data.error || data.detail)) || '').trim();
    if (!raw) return fallback;
    if (raw === 'invalid credentials') return 'E-posta veya şifre hatalı.';
    if (raw === 'database unavailable') return 'Veritabanı bağlantısı hazır değil.';
    if (raw === 'token signing failed') return 'Oturum anahtarı eksik. USER_SESSION_SECRET veya JWT_SECRET ekleyin.';
    return raw;
  }

  async function verifyMe(token) {
    const response = await fetch('/api/me', { headers: { Authorization: 'Bearer ' + token } });
    if (response.status === 401 || response.status === 403) clearJwt();
    return readJSON(response);
  }

  async function backendAuth(path, body) {
    const response = await fetch(path, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      credentials: 'same-origin',
      body: JSON.stringify(body || {})
    });
    const data = await readJSON(response);
    if (!response.ok) throw new Error(message(data, 'Auth işlemi başarısız.'));
    const token = findJwt(data);
    if (!isJwt(token)) throw new Error('Giriş oturumu alınamadı.');
    saveJwt(token);
    await verifyMe(token).catch(() => null);
    return { ...data, access_token: token, token_type: 'Bearer' };
  }

  async function init() {
    const token = getJwt();
    if (isJwt(token) && isJwtExpired(token)) clearJwt();
  }

  async function signUp(email, password) {
    return backendAuth('/api/auth/register', { email: String(email || '').trim(), password, name: String(email || '').split('@')[0] || 'Kullanıcı' });
  }

  async function signIn(email, password) {
    return backendAuth('/api/auth/login', { email: String(email || '').trim(), password });
  }

  async function signOut() {
    clearJwt();
    window.location.href = '/login.html';
  }

  function isLoggedIn() {
    const token = getJwt();
    if (!isJwt(token)) return false;
    if (isJwtExpired(token)) {
      clearJwt();
      return false;
    }
    return true;
  }

  function requireAuth() {
    if (!isLoggedIn()) {
      window.location.href = '/login.html';
      return false;
    }
    return true;
  }

  async function apiCall(path, options = {}) {
    const token = getJwt();
    const headers = new Headers(options.headers || {});
    if (isJwt(token) && !isJwtExpired(token) && !headers.has('Authorization')) headers.set('Authorization', 'Bearer ' + token);
    try {
      const response = await fetch(path, { ...options, headers });
      if (response.status === 401 || response.status === 403) clearJwt();
      return response;
    } catch {
      return null;
    }
  }

  function consumeAccessTokenFromHash() { return false; }

  window.KoscheiAuth = { init, signIn, signUp, signOut, consumeAccessTokenFromHash, isLoggedIn, requireAuth, apiCall, getEmail, getSub, getJwt, clearJwt, isJwtExpired };
})();
