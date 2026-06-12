(function () {
  'use strict';

  const TOKEN_KEY = 'koschei_jwt';
  const LEGACY_TOKEN_KEY = 'koschei_token';

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
      payload.session && payload.session.access_token,
      payload.data && payload.data.token,
      payload.data && payload.data.access_token,
      payload.data && payload.data.session && payload.data.session.access_token,
    ];
    return candidates.find(isJWT) || candidates.find(Boolean) || '';
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

  async function authRequest(path, body) {
    const response = await fetch(path, {
      method: 'POST',
      credentials: 'same-origin',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(body),
    });
    const data = await parseJSON(response);
    if (!response.ok) {
      throw new Error(errorMessage(data, `Authentication failed (${response.status})`));
    }
    const token = extractToken(data);
    if (!token) {
      throw new Error('Authentication succeeded, but the server did not return a token.');
    }
    saveToken(token);
    return data;
  }

  async function login(email, password) {
    return authRequest('/api/auth/login', { email, password });
  }

  async function register(email, password, name) {
    return authRequest('/api/auth/register', { email, password, name });
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
