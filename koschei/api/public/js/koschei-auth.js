(function () {
  const KEY = 'koschei_jwt';
  const LEGACY_KEY = 'koschei_token';

  function saveJwt(t) {
    try {
      if (t) {
        localStorage.setItem(KEY, t);
        localStorage.setItem(LEGACY_KEY, t);
      }
    } catch {}
  }

  function getJwt() {
    try { return localStorage.getItem(KEY) || localStorage.getItem(LEGACY_KEY) || ''; } catch { return ''; }
  }

  function clearJwt() {
    try {
      localStorage.removeItem(KEY);
      localStorage.removeItem(LEGACY_KEY);
    } catch {}
  }

  function _isJwt(t) {
    if (!t || typeof t !== 'string') return false;
    const p = t.split('.');
    return p.length === 3 && p.every(s => s.length > 0);
  }

  function _b64url(s) {
    s = String(s || '').replace(/-/g, '+').replace(/_/g, '/');
    while (s.length % 4) s += '=';
    try { return atob(s); } catch { return ''; }
  }

  function jwtPayload(jwt) {
    try { return JSON.parse(_b64url(String(jwt || '').split('.')[1] || '')); } catch { return {}; }
  }

  function jwtIsUsable(jwt) {
    if (!_isJwt(jwt)) return false;
    const payload = jwtPayload(jwt);
    const exp = Number(payload.exp || 0);
    if (!exp) return true;
    return exp > Math.floor(Date.now() / 1000) + 20;
  }

  function getEmail() {
    const jwt = getJwt();
    if (!jwt) return null;
    return jwtPayload(jwt).email || null;
  }

  function getSub() {
    const jwt = getJwt();
    if (!jwt) return null;
    return jwtPayload(jwt).sub || null;
  }

  function fallbackMeFromJwt(jwt) {
    const payload = jwtPayload(jwt);
    const email = payload.email || '';
    const sub = payload.sub || email;
    return {
      ok: true,
      user: {
        auth_subject: sub,
        email,
        role: 'member',
        plan_id: 'free',
        plan: 'free',
        credits: 0,
        outputs_total: 0,
        outputs_remaining: 0,
      },
      warning: 'profile_database_unavailable',
    };
  }

  function jsonResponse(body, status) {
    return new Response(JSON.stringify(body), {
      status: status || 200,
      headers: { 'Content-Type': 'application/json' },
    });
  }

  function installDatabaseFallbackFetch() {
    if (window.__koscheiDatabaseFallbackFetchInstalled) return;
    window.__koscheiDatabaseFallbackFetchInstalled = true;
    const nativeFetch = window.fetch.bind(window);
    window.fetch = async function(input, init) {
      let path = '';
      let sameOrigin = false;
      try {
        const raw = typeof input === 'string' ? input : (input && input.url) || '';
        const url = new URL(raw, window.location.origin);
        path = url.pathname;
        sameOrigin = url.origin === window.location.origin;
      } catch {}

      let requestInit = init;
      if (sameOrigin && path.startsWith('/api/')) {
        const jwt = getJwt();
        if (jwtIsUsable(jwt)) {
          const headers = new Headers((requestInit && requestInit.headers) || (input && input.headers) || {});
          if (!headers.has('Authorization')) headers.set('Authorization', 'Bearer ' + jwt);
          requestInit = { ...(requestInit || {}), headers };
        }
      }

      const res = await nativeFetch(input, requestInit);
      if (!path.startsWith('/api/')) return res;
      if (res.status !== 503) return res;
      const clone = res.clone();
      let data = {};
      try { data = await clone.json(); } catch {}
      const rawError = String(data.error || data.message || '').toLowerCase();
      if (!rawError.includes('database unavailable')) return res;
      if (path === '/api/me/package') {
        return jsonResponse({
          success: true,
          code: 'OK',
          data: { has_active_package: false, plan_id: null, status: 'none', expires_at: null, warning: 'package_database_unavailable' },
        }, 200);
      }
      if (path === '/api/me') {
        return jsonResponse(fallbackMeFromJwt(getJwt()), 200);
      }
      if (path === '/api/v1/unified/analyze') {
        return jsonResponse({
          success: false,
          code: 'SERVICE_TEMPORARILY_UNAVAILABLE',
          message: 'Koschei analiz servisi şu an veritabanı bağlantısını bekliyor. Ekran çalışıyor; analiz için backend DB bağlantısı aktif olmalı.',
          data: null,
        }, 503);
      }
      return jsonResponse({
        success: false,
        code: 'SERVICE_TEMPORARILY_UNAVAILABLE',
        message: 'Koschei servisi şu an veritabanı bağlantısını bekliyor.',
        data: null,
      }, 503);
    };
  }

  installDatabaseFallbackFetch();

  function defaultUserName(email) {
    const name = String(email || '').split('@')[0].trim();
    return name || 'Kullanıcı';
  }

  function cleanInternalPath(value, fallback) {
    const raw = String(value || '').trim();
    if (!raw || !raw.startsWith('/')) return fallback || '/';
    if (raw.startsWith('//')) return fallback || '/';
    if (/^\/\/[a-z0-9]/i.test(raw)) return fallback || '/';
    if (raw.startsWith('/login')) return fallback || '/';
    return raw;
  }

  function currentReturnPath() {
    return cleanInternalPath(window.location.pathname + window.location.search + window.location.hash, '/');
  }

  function nextPath(fallback) {
    try {
      const params = new URLSearchParams(window.location.search || '');
      return cleanInternalPath(params.get('next'), fallback || '/');
    } catch { return fallback || '/'; }
  }

  function loginURL(base) {
    const loginPath = String(base || '/login.html').trim() || '/login.html';
    const normalized = loginPath === '/login' ? '/login.html' : loginPath;
    const sep = normalized.includes('?') ? '&' : '?';
    return normalized + sep + 'next=' + encodeURIComponent(currentReturnPath());
  }

  function successCallbackURL() {
    return window.location.origin.replace(/\/+$/, '') + '/hub.html';
  }

  function publicErrorMessage(raw, fallback) {
    const value = String(raw || '').trim();
    const normalized = value.toLowerCase();
    if (!value) return fallback;
    if (['unauthorized', 'token_missing', 'auth_session_missing', 'auth_verification_required'].includes(normalized) || normalized.includes('401')) return 'Giriş yapmanız gerekiyor.';
    if (['forbidden', 'insufficient_outputs'].includes(normalized) || normalized.includes('active package') || normalized.includes('active_entitlement_required')) return 'Bu işlem için aktif Koschei paketi gerekli.';
    if (normalized.includes('database unavailable')) return 'Koschei veritabanı bağlantısı geçici olarak beklemede.';
    if (normalized.includes('paddle')) return value;
    if (normalized.includes('shopier')) return 'Shopier bağlantısı açılamadı.';
    return value || fallback;
  }

  function errorMessage(data, fallback) {
    if (!data) return fallback;
    if (typeof data === 'string') return publicErrorMessage(data, fallback);
    if (data.error === 'token_missing') return 'Giriş oturumu alınamadı. Lütfen tekrar giriş yapın.';
    if (data.error === 'auth_session_missing' || data.error === 'auth_verification_required') {
      return publicErrorMessage(data.message, 'Giriş oturumu alınamadı. Lütfen tekrar giriş yapın.');
    }
    return publicErrorMessage(data.message || data.error_description || data.error || data.detail, fallback);
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
          if (!res.ok) throw new Error(errorMessage(data, 'Kimlik doğrulama yapılandırması şu anda kullanılamıyor.'));
          return data;
        });
    }
    return configPromise;
  }

  async function neonAuthBaseURL() {
    const cfg = await loadConfig();
    const baseURL = String(cfg.neonAuthUrl || '').trim().replace(/\/+$/, '');
    if (!baseURL) throw new Error('Neon Auth yapılandırılmamış.');
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
      const raw = String(data.error || data.message || '').toLowerCase();
      if (res.status === 503 && raw.includes('database unavailable')) {
        return fallbackMeFromJwt(jwt);
      }
      clearJwt();
      throw new Error(errorMessage(data, 'Giriş yapmanız gerekiyor.'));
    }
    return data;
  }

  async function finishAuth(result) {
    const jwt = jwtFromHeader(result.headerJwt) || findJwt(result.data);
    if (!_isJwt(jwt)) throw new Error('Giriş oturumu alınamadı. Lütfen tekrar giriş yapın.');
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

  async function restoreNeonSession() {
    try {
      const baseURL = await neonAuthBaseURL();
      const session = await fetchNeonSession(baseURL);
      if (!session) return false;
      await finishAuth(session);
      return true;
    } catch {
      return false;
    }
  }

  async function init() {
    consumeAccessTokenFromHash();
    try { await loadConfig(); } catch {}

    const jwt = getJwt();
    if (jwtIsUsable(jwt)) {
      try {
        await verifyMe(jwt);
        return true;
      } catch {}
    } else if (jwt) {
      clearJwt();
    }

    return restoreNeonSession();
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

  function isLoggedIn() { return jwtIsUsable(getJwt()); }

  function requireAuth(loginPath) {
    if (!isLoggedIn()) {
      window.location.href = loginURL(loginPath || '/login.html');
      return false;
    }
    return true;
  }

  async function apiCall(path, options = {}) {
    const jwt = getJwt();
    const headers = new Headers(options.headers || {});
    if (jwtIsUsable(jwt) && !headers.has('Authorization')) headers.set('Authorization', 'Bearer ' + jwt);
    try {
      return await fetch(path, { ...options, headers });
    } catch { return null; }
  }

  window.KoscheiAuth = {
    init,
    signIn,
    signUp,
    signOut,
    consumeAccessTokenFromHash,
    isLoggedIn,
    requireAuth,
    apiCall,
    getEmail,
    getSub,
    getJwt,
    nextPath,
    loginURL,
    restoreNeonSession,
  };
})();

// Auth helper only: Neon Auth session restore + JWT persistence. It must not mutate or lock radar UI DOM.
