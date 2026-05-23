const NEON_AUTH_URL = (process.env.EXPO_PUBLIC_NEON_AUTH_URL || '').trim();

type AuthResponse = {
  access_token?: string;
  token?: string;
  session?: { access_token?: string };
  user?: { email?: string };
  error?: { message?: string; code?: string };
  message?: string;
};

function authUnavailable() {
  return new Error('auth service unavailable');
}

function resolveToken(payload: AuthResponse): string {
  return payload.access_token || payload.token || payload.session?.access_token || '';
}

async function callNeonAuth(path: string, body: { email: string; password: string }) {
  if (!NEON_AUTH_URL) throw authUnavailable();

  const res = await fetch(`${NEON_AUTH_URL}${path}`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(body),
  }).catch(() => {
    throw authUnavailable();
  });

  const payload = (await res.json().catch(() => ({}))) as AuthResponse;
  if (!res.ok) {
    const msg = (payload?.error?.message || payload?.message || '').toLowerCase();
    throw new Error(msg);
  }
  return payload;
}

export const neonAuth = {
  async signUpWithEmail(email: string, password: string) {
    return callNeonAuth('/signup/email', { email, password });
  },
  async signInWithEmail(email: string, password: string) {
    return callNeonAuth('/signin/email', { email, password });
  },
  tokenFrom(payload: AuthResponse) {
    return resolveToken(payload);
  },
};
