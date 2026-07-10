(function () {
  'use strict';

  const KEY = 'koschei_jwt';
  const LEGACY_KEY = 'koschei_token';

  function saveJwt(token) {
    try {
      if (!token) return;
      localStorage.setItem(KEY, token);
      localStorage.setItem(LEGACY_KEY, token);
    } catch {}
  }

  function getJwt() {
    try { return localStorage.getItem(KEY) || localStorage.getItem(LEGACY_KEY) || ''; }
    catch { return ''; }
  }

  function clearJwt() {
    try {
      localStorage.removeItem(KEY);
      localStorage.removeItem(LEGACY_KEY);
    } catch {}
  }

  function isJwt(token) {
    return typeof token === 'string' && token.split('.').length === 3 && token.split('.').every(Boolean);
  }

  function decodePart(value) {
    let input = String(value || '').replace(/-/g, '+').replace(/_/g, '/');
    while (input.length % 4) input += '=';
    try { return atob(input); } catch { return ''; }
  }

  function jwtPayload(token) {
    try { return JSON.parse(decodePart(String(token || '').split('.')[1] || '')); }
    catch { return {}; }
  }

  function jwtIsUsable(token) {
    if (!isJwt(token)) return false;
    const exp = Number(jwtPayload(token).exp || 0);
    return !exp || exp > Math.floor(Date.now() / 1000) + 20;
  }

  function getEmail() { return jwtPayload(getJwt()).email || null; }
  function getSub() { return jwtPayload(getJwt()).sub || null; }

  function installAuthenticatedFetch() {
    if (window.__koscheiAuthenticatedFetchInstalled) return;
    window.__koscheiAuthenticatedFetchInstalled = true;
    const nativeFetch = window.fetch.bind(window);
    window.fetch = function (input, init) {
      let requestInit = init;
      try {
        const raw = typeof input === 'string' ? input : (input && input.url) || '';
        const url = new URL(raw, window.location.origin);
        if (url.origin === window.location.origin && url.pathname.startsWith('/api/')) {
          const token = getJwt();
          if (jwtIsUsable(token)) {
            const headers = new Headers((init && init.headers) || (input && input.headers) || {});
            if (!headers.has('Authorization')) headers.set('Authorization', 'Bearer ' + token);
            requestInit = { ...(init || {}), headers };
          }
        }
      } catch {}
      return nativeFetch(input, requestInit);
    };
  }

  installAuthenticatedFetch();

  function defaultUserName(email) {
    return String(email || '').split('@')[0].trim() || 'Kullanıcı';
  }

  function cleanInternalPath(value, fallback = '/') {
    const raw = String(value || '').trim();
    if (!raw.startsWith('/') || raw.startsWith('//') || raw.startsWith('/login')) return fallback;
    return raw;
  }

  function currentReturnPath() {
    return cleanInternalPath(location.pathname + location.search + location.hash, '/');
  }

  function nextPath(fallback = '/') {
    try { return cleanInternalPath(new URLSearchParams(location.search).get('next'), fallback); }
    catch { return fallback; }
  }

  function loginURL(base = '/login.html') {
    const path = String(base || '/login.html').trim() === '/login' ? '/login.html' : String(base || '/login.html').trim();
    return path + (path.includes('?') ? '&' : '?') + 'next=' + encodeURIComponent(currentReturnPath());
  }

  function successCallbackURL() {
    return location.origin.replace(/\/+$/, '') + '/dashboard';
  }

  function publicErrorMessage(raw, fallback) {
    const value = String(raw || '').trim();
    const normalized = value.toLowerCase();
    if (!value) return fallback;
    if (['unauthorized', 'token_missing', 'auth_session_missing', 'auth_verification_required'].includes(normalized) || normalized.includes('401')) return 'Giriş yapmanız gerekiyor.';
    if (['forbidden', 'insufficient_outputs', 'kosch_holder_required', 'active_entitlement_required'].includes(normalized) || normalized.includes('active package') || normalized.includes('verified kosch holder')) return 'Bu işlem için doğrulanmış KOSCH holder access gerekli.';
    if (normalized.includes('database unavailable') || normalized.includes('could not be verified') || normalized.includes('kosch_access_unavailable')) return 'KOSCH erişim doğrulaması geçici olarak kullanılamıyor.';
    return value || fallback;
  }

  function errorMessage(data, fallback) {
    if (!data) return fallback;
    if (typeof data === 'string') return publicErrorMessage(data, fallback);
    if (data.error === 'token_missing') return 'Giriş oturumu alınamadı. Lütfen tekrar giriş yapın.';
    return publicErrorMessage(data.message || data.error_description || data.error || data.detail, fallback);
  }

  async function readJSON(response) {
    const text = await response.text().catch(() => '');
    if (!text) return {};
    try { return JSON.parse(text); } catch { return { message: text }; }
  }

  function jwtFromHeader(value) {
    const token = String(value || '').replace(/^Bearer\s+/i, '').trim();
    return isJwt(token) ? token : '';
  }

  function findJwt(value) {
    if (!value || typeof value !== 'object') return '';
    const candidates = [
      value.token, value.jwt, value.access_token, value.id_token, value.auth_token,
      value.data && value.data.token, value.data && value.data.jwt,
      value.data && value.data.access_token, value.data && value.data.id_token,
      value.session && value.session.token, value.session && value.session.jwt,
      value.session && value.session.access_token, value.session && value.session.id_token,
    ];
    return candidates.find(isJwt) || '';
  }

  let configPromise;
  function loadConfig() {
    if (!configPromise) {
      configPromise = fetch('/api/config', { credentials: 'same-origin' }).then(async response => {
        const data = await readJSON(response);
        if (!response.ok) throw new Error(errorMessage(data, 'Kimlik doğrulama yapılandırması kullanılamıyor.'));
        return data;
      });
    }
    return configPromise;
  }

  async function neonAuthBaseURL() {
    const config = await loadConfig();
    const baseURL = String(config.neonAuthUrl || '').trim().replace(/\/+$/, '');
    if (!baseURL) throw new Error('Neon Auth yapılandırılmamış.');
    return baseURL;
  }

  async function parseNeonResponse(response) {
    return {
      data: await readJSON(response),
      headerJwt: response.headers.get('set-auth-jwt') || response.headers.get('authorization') || '',
    };
  }

  async function fetchNeonJSON(baseURL, path, options = {}) {
    // HARD RULE: never add credentials:'include' here. Neon Auth is
    // cross-origin and credentialed requests trigger the recurring CORS lock.
    // Authentication tokens travel in response bodies/headers, not cookies.
    const response = await fetch(baseURL + path, { ...options });
    const result = await parseNeonResponse(response);
    if (!response.ok) throw new Error(errorMessage(result.data, `Neon Auth failed (${response.status})`));
    return result;
  }

  async function fetchNeonSession(baseURL) {
    for (const [method, path] of [['GET','/token'], ['GET','/get-session'], ['POST','/token'], ['POST','/get-session']]) {
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

  async function verifyMe(token) {
    const response = await fetch('/api/me', {
      method: 'GET',
      credentials: 'same-origin',
      headers: { Authorization: 'Bearer ' + token },
    });
    const data = await readJSON(response);
    if (!response.ok) {
      if (response.status === 401) clearJwt();
      throw new Error(errorMessage(data, response.status === 503 ? 'Koschei veritabanı bağlantısı geçici olarak kullanılamıyor.' : 'Giriş yapmanız gerekiyor.'));
    }
    return data;
  }

  async function finishAuth(result) {
    const token = jwtFromHeader(result.headerJwt) || findJwt(result.data);
    if (!isJwt(token)) throw new Error('Giriş oturumu alınamadı. Lütfen tekrar giriş yapın.');
    saveJwt(token);
    return { ...result.data, me: await verifyMe(token), access_token: token, token_type: 'Bearer' };
  }

  function consumeAccessTokenFromHash() {
    const params = new URLSearchParams((location.hash || '').replace(/^#/, ''));
    const token = params.get('access_token') || params.get('token') || params.get('id_token') || '';
    if (!isJwt(token)) return false;
    saveJwt(token);
    params.delete('access_token');
    params.delete('token');
    params.delete('id_token');
    history.replaceState(null, document.title, location.pathname + location.search + (params.toString() ? '#' + params : ''));
    return true;
  }

  async function restoreNeonSession() {
    try {
      const session = await fetchNeonSession(await neonAuthBaseURL());
      if (!session) return false;
      await finishAuth(session);
      return true;
    } catch { return false; }
  }

  async function init() {
    consumeAccessTokenFromHash();
    try { await loadConfig(); } catch {}
    const token = getJwt();
    if (jwtIsUsable(token)) {
      try { await verifyMe(token); return true; } catch {}
    } else if (token) {
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
    return finishAuth(session || result);
  }

  async function signUp(email, password) {
    try {
      return await neonEmailAuth('/sign-up/email', { email, password, name: defaultUserName(email), callbackURL: successCallbackURL() });
    } catch (error) {
      if (!String(error && error.message || '').includes('Giriş oturumu alınamadı')) throw error;
      return signIn(email, password);
    }
  }

  function signIn(email, password) {
    return neonEmailAuth('/sign-in/email', { email, password, callbackURL: successCallbackURL() });
  }

  function signOut() {
    clearJwt();
    location.href = '/login.html';
  }

  function isLoggedIn() { return jwtIsUsable(getJwt()); }

  function requireAuth(loginPath) {
    if (isLoggedIn()) return true;
    location.href = loginURL(loginPath || '/login.html');
    return false;
  }

  async function apiCall(path, options = {}) {
    const headers = new Headers(options.headers || {});
    const token = getJwt();
    if (jwtIsUsable(token) && !headers.has('Authorization')) headers.set('Authorization', 'Bearer ' + token);
    try { return await fetch(path, { ...options, headers }); }
    catch { return null; }
  }

  window.KoscheiAuth = {
    init, signIn, signUp, signOut, consumeAccessTokenFromHash, isLoggedIn,
    requireAuth, apiCall, getEmail, getSub, getJwt, nextPath, loginURL,
    restoreNeonSession,
  };
})();

// Auth helper only: Neon Auth session restore + JWT persistence.
// It must not mutate or lock Radar UI DOM.
