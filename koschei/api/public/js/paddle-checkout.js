// Legacy checkout client intentionally retired.
// Koschei ARVIS has no Paddle, Shopier, card or transfer-proof flow.
// Product access is derived only from verified KOSCH holder balance.
window.KoscheiCheckout = Object.freeze({
  enabled: false,
  provider: 'kosch_token',
  open() { window.location.assign('/kosch-access'); }
});
