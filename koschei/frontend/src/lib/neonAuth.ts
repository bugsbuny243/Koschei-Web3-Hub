const NEON_AUTH_URL = (process.env.EXPO_PUBLIC_NEON_AUTH_URL || '').trim();

async function callNeonAuth(path: string, body: { email: string; password: string }) {
  if (!NEON_AUTH_URL) throw new Error('auth service unavailable');
  const res = await fetch(NEON_AUTH_URL + path, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(body),
  }).catch(() => { throw new Error('auth service unavailable'); });
  const payload: any = await res.json().catch(() => ({}));
  if (!res.ok) throw new Error(payload?.error?.message || 'auth service unavailable');
  return payload;
}

export const neonAuth = {
  async signUpWithEmail(email: string, password: string) {
    return callNeonAuth('/api/auth/sign-up/email', { email, password });
  },
  async signInWithEmail(email: string, password: string) {
    return callNeonAuth('/api/auth/sign-in/email', { email, password });
  },
  tokenFrom(payload: any): string {
    return payload?.token || payload?.access_token || payload?.session?.access_token || '';
  },
};
