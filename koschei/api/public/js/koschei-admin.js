(() => {
  const storageKey = 'koschei_admin_password';

  window.adminPassword = () => sessionStorage.getItem(storageKey) || '';
  window.isAdminUnlocked = () => window.adminPassword() !== '';
  window.unlockAdmin = password => {
    if (!password) throw new Error('Enter admin password');
    sessionStorage.setItem(storageKey, password);
  };
  window.lockAdmin = () => sessionStorage.removeItem(storageKey);
  window.adminFetch = async (path, options = {}) => {
    const password = window.adminPassword();
    if (!password) {
      const error = new Error('Enter admin password');
      error.code = 'ADMIN_LOCKED';
      throw error;
    }

    const headers = new Headers(options.headers || {});
    headers.set('x-admin-password', password);
    if (options.body && !headers.has('Content-Type')) headers.set('Content-Type', 'application/json');
    return fetch(path, { ...options, headers });
  };
})();
