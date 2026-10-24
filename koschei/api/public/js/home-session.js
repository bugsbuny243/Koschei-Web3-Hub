// Reflects a verified account session in the public home header.
// Presentation only: it never grants access; KoscheiAuth.init verifies the
// session with /api/me and an unverified or missing session keeps "Sign in".
(async () => {
  'use strict';
  const auth = window.KoscheiAuth;
  const link = document.querySelector('[data-home-session-link]');
  if (!auth || !link) return;
  try { await auth.init(); } catch { return; }
  if (!auth.isLoggedIn()) return;
  link.href = '/account';
  link.textContent = 'Account';
})();
