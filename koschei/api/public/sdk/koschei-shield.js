/* Koschei Shield SDK - browser-safe client. Do not embed server API keys in public frontend apps. */
(function (global) {
  function KoscheiShield(options) {
    options = options || {};
    this.baseURL = String(options.baseURL || '').replace(/\/+$/, '') || window.location.origin;
    this.apiKey = options.apiKey || '';
    this.getToken = options.getToken || null;
  }

  KoscheiShield.prototype._headers = async function () {
    var headers = { 'Content-Type': 'application/json' };
    var token = this.apiKey;
    if (!token && typeof this.getToken === 'function') token = await this.getToken();
    if (token) headers['X-API-Key'] = token;
    return headers;
  };

  KoscheiShield.prototype.preflight = async function (payload) {
    var res = await fetch(this.baseURL + '/api/v1/shield/preflight', {
      method: 'POST',
      headers: await this._headers(),
      body: JSON.stringify(payload || {})
    });
    var data = await res.json().catch(function () { return {}; });
    if (!res.ok) {
      var err = new Error(data.message || data.error || 'Koschei Shield request failed');
      err.response = data;
      err.status = res.status;
      throw err;
    }
    return data;
  };

  KoscheiShield.prototype.transaction = async function (payload) {
    var res = await fetch(this.baseURL + '/api/v1/shield/transaction', {
      method: 'POST',
      headers: await this._headers(),
      body: JSON.stringify(payload || {})
    });
    var data = await res.json().catch(function () { return {}; });
    if (!res.ok) {
      var err = new Error(data.message || data.error || 'Koschei Shield request failed');
      err.response = data;
      err.status = res.status;
      throw err;
    }
    return data;
  };

  KoscheiShield.prototype.assertSafe = async function (payload) {
    var verdict = await this.preflight(payload);
    // Explicit allow-only contract. "withhold", unknown evidence, unsigned or
    // malformed responses MUST NOT authorize a wallet transaction.
    var allowed = verdict && verdict.ok === true &&
      (verdict.action === 'allow' || verdict.action === 'allow_with_monitoring') &&
      verdict.signed === true && typeof verdict.signature === 'string' && verdict.signature.length > 0 &&
      verdict.final_policy !== 'withhold' &&
      typeof verdict.risk_level === 'string' &&
      ['low', 'medium'].indexOf(verdict.risk_level.toLowerCase()) !== -1;
    if (!allowed) {
      var err = new Error('Koschei Shield: transaction not authorized - ' + ((verdict && (verdict.reason || verdict.verdict)) || 'missing verified allow decision'));
      err.verdict = verdict;
      throw err;
    }
    return verdict;
  };

  global.KoscheiShield = KoscheiShield;
})(typeof window !== 'undefined' ? window : globalThis);
