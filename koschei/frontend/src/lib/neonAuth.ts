const NEON_AUTH_URL = (process.env.EXPO_PUBLIC_NEON_AUTH_URL || '').trim().replace(/\/+$/, '');

function getNeonAuthUrl(): string {
  if (!NEON_AUTH_URL) {
    throw new Error('auth_config_missing: EXPO_PUBLIC_NEON_AUTH_URL is empty');
  }
  return NEON_AUTH_URL;
}

async function post(path: string, body: Record<string, unknown>): Promise<any> {
  const baseUrl = getNeonAuthUrl();
  let res: Response;

  try {
    res = await fetch(`${baseUrl}${path}`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      credentials: 'include',
      body: JSON.stringify(body),
    });
  } catch {
    throw new Error('auth service unavailable');
  }

  const payload: any = await res.json().catch(() => ({}));

  if (!res.ok) {
    const msg =
      payload?.error?.message ||
      payload?.message ||
      payload?.error ||
      `auth request failed (${res.status})`;
    throw new Error(`${msg} (status ${res.status})`);
  }

  return payload;
}

export const neonAuth = {
  async signUpWithEmail(email: string, password: string) {
    return post('/auth/sign-up/email', {
      email,
      password,
      name: email.split('@')[0] || 'User',
    });
  },

  async signInWithEmail(email: string, password: string) {
    return post('/auth/sign-in/email', { email, password });
  },

  async signOut() {
    return;
  },

  tokenFrom(payload: any): string {
    return (
      payload?.data?.session?.access_token ||
      payload?.session?.access_token ||
      payload?.data?.access_token ||
      payload?.access_token ||
      payload?.token ||
      ''
    );
  },
};
