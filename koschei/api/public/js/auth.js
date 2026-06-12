(function () {
  'use strict';

  const TOKEN_KEY = 'koschei_jwt';
  const LEGACY_TOKEN_KEY = 'koschei_token';
  let configPromise;

  function safeLocalStorage(callback, fallback) {
    try {
      return callback(window.localStorage);
    } catch {
      return fallback;
    }
  }

  function isJWT(value) {
    return typeof value === 'string' && value.split('.').length === 3;
  }

  function saveToken(token) {
    if (!token) return;
    safeLocalStorage((storage) => {
      storage.setItem(TOKEN_KEY, token);
      // Keep the retired key in sync so older pages can read sessions created by auth.js.
      storage.setItem(LEGACY_TOKEN_KEY, token);
    });
  }

  function getToken() {
    return safeLocalStorage((storage) => storage.getItem(TOKEN_KEY) || storage.getItem(LEGACY_TOKEN_KEY) || '', '');
  }

  function clearToken() {
    safeLocalStorage((storage) => {
      storage.removeItem(TOKEN_KEY);
      storage.removeItem(LEGACY_TOKEN_KEY);
    });
  }

  function getAuthHeader() {
    const token = getToken();
    return token ? `Bearer ${token}` : '';
  }

  function extractToken(payload) {
    if (!payload || typeof payload !== 'object') return '';
    const candidates = [
      payload.token,
      payload.access_token,
      payload.jwt,
      payload.id_token,
      payload.auth_token,
      payload.session && payload.session.token,
      payload.session && payload.session.jwt,
      payload.session && payload.session.access_token,
      payload.session && payload.session.id_token,
      payload.data && payload.data.token,
      payload.data && payload.data.jwt,
      payload.data && payload.data.access_token,
      payload.data && payload.data.id_token,
      payload.data && payload.data.session && payload.data.session.token,
      payload.data && payload.data.session && payload.data.session.jwt,
      payload.data && payload.data.session && payload.data.session.access_token,
      payload.data && payload.data.session && payload.data.session.id_token,
    ];
    return candidates.find(isJWT) || '';
  }

  async function parseJSON(response) {
    const text = await response.text().catch(() => '');
    if (!text) return {};
    try {
      return JSON.parse(text);
    } catch {
      return { message: text };
    }
  }

  function errorMessage(payload, fallback) {
    if (!payload) return fallback;
    return payload.message || payload.error_description || payload.error || payload.detail || fallback;
  }

  async function loadConfig() {
    if (!configPromise) {
      configPromise = fetch('/api/config', { credentials: 'same-origin' })
        .then(async (response) => {
          const data = await parseJSON(response);
          if (!response.ok) throw new Error(errorMessage(data, 'Auth configuration is unavailable.'));
          return data;
        });
    }
    return configPromise;
  }

  async function neonAuthBaseURL() {
    const config = await loadConfig();
    const baseURL = String(config.neonAuthUrl || '').trim().replace(/\/+$/, '');
    if (!baseURL) throw new Error('Neon Auth is not configured.');
    return baseURL;
  }

  function tokenFromHeader(value) {
    const token = String(value || '').replace(/^Bearer\s+/i, '').trim();
    return isJWT(token) ? token : '';
  }

  async function parseNeonResponse(response) {
    const data = await parseJSON(response);
    const headerToken = response.headers.get('set-auth-jwt') || response.headers.get('authorization') || '';
    return { data, token: tokenFromHeader(headerToken) || extractToken(data) };
  }

  async function fetchNeonJSON(baseURL, path, options = {}) {
    const response = await fetch(baseURL + path, { credentials: 'include', ...options });
    const result = await parseNeonResponse(response);
    if (!response.ok) {
      throw new Error(errorMessage(result.data, `Neon Auth failed (${response.status})`));
    }
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
        if (isJWT(result.token)) return result;
      } catch {}
    }
    return null;
  }

  async function verifyWithBackend(token) {
    const response = await fetch('/api/me', {
      method: 'GET',
      credentials: 'same-origin',
      headers: { Authorization: `Bearer ${token}` },
    });
    const data = await parseJSON(response);
    if (!response.ok) {
      clearToken();
      throw new Error(errorMessage(data, 'Token was received, but /api/me rejected it.'));
    }
    return data;
  }

  async function finishAuth(result) {
    const token = result && result.token;
    if (!isJWT(token)) {
      throw new Error('Authentication succeeded, but Neon Auth did not return a token.');
    }
    saveToken(token);
    const me = await verifyWithBackend(token);
    return { ...result.data, me, access_token: token, token_type: 'Bearer' };
  }

  async function neonEmailAuth(path, body) {
    const baseURL = await neonAuthBaseURL();
    const result = await fetchNeonJSON(baseURL, path, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(body),
    });
    if (isJWT(result.token)) return finishAuth(result);
    const session = await fetchNeonSession(baseURL);
    if (session) return finishAuth(session);
    return finishAuth(result);
  }

  function successCallbackURL() {
    return window.location.origin.replace(/\/+$/, '') + '/hub.html';
  }

  function defaultName(email) {
    const name = String(email || '').split('@')[0].trim();
    return name || 'User';
  }

  async function login(email, password) {
    return neonEmailAuth('/sign-in/email', { email, password, callbackURL: successCallbackURL() });
  }

  async function register(email, password, name) {
    return neonEmailAuth('/sign-up/email', { email, password, name: name || defaultName(email), callbackURL: successCallbackURL() });
  }

  async function checkAuth(options = {}) {
    const redirect = options.redirect !== false;
    const loginPath = options.loginPath || '/login';
    const token = getToken();
    if (!token) {
      if (redirect) window.location.replace(loginPath);
      return false;
    }

    const response = await fetch('/api/me', {
      method: 'GET',
      credentials: 'same-origin',
      headers: { Authorization: getAuthHeader() },
    }).catch(() => null);

    if (!response || !response.ok) {
      clearToken();
      if (redirect) window.location.replace(loginPath);
      return false;
    }
    return true;
  }

  function logout(redirectTo = '/') {
    clearToken();
    window.location.href = redirectTo;
  }

  window.Auth = { login, register, checkAuth, logout, getAuthHeader, getToken, saveToken, clearToken };
  window.login = login;
  window.checkAuth = checkAuth;
  window.logout = logout;
  window.getAuthHeader = getAuthHeader;
})();
