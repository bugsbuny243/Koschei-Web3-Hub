const NEON_AUTH_BASE_URL = (process.env.EXPO_PUBLIC_NEON_AUTH_BASE_URL || '').trim();
const NEON_AUTH_SIGNIN_PATH = (process.env.EXPO_PUBLIC_NEON_AUTH_SIGNIN_PATH || '/email/sign-in').trim();
const NEON_AUTH_SIGNUP_PATH = (process.env.EXPO_PUBLIC_NEON_AUTH_SIGNUP_PATH || '/email/sign-up').trim();

function getTokenFromResponse(data: any): string {
  return data?.token || data?.access_token || data?.session?.access_token || data?.session_token || '';
}

async function neonRequest(path: string, body: { email: string; password: string }) {
  if (!NEON_AUTH_BASE_URL) throw new Error('auth service unavailable');
  const res = await fetch(`${NEON_AUTH_BASE_URL}${path}`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(body),
  });
  const data = await res.json().catch(() => ({}));
  if (!res.ok) {
    const raw = String(data?.error || data?.message || '').toLowerCase();
    if (raw.includes('exist') || raw.includes('already')) throw new Error('account already exists');
    if (raw.includes('invalid') || raw.includes('credential') || raw.includes('password')) throw new Error('invalid email/password');
    throw new Error('auth service unavailable');
  }
  const token = getTokenFromResponse(data);
  if (!token) throw new Error('auth service unavailable');
  return token;
}

export const neonAuth = {
  signInWithEmail: (email: string, password: string) => neonRequest(NEON_AUTH_SIGNIN_PATH, { email, password }),
  signUpWithEmail: (email: string, password: string) => neonRequest(NEON_AUTH_SIGNUP_PATH, { email, password }),
};
