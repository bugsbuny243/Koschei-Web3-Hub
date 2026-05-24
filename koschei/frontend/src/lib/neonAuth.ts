const NEON_AUTH_URL = (process.env.EXPO_PUBLIC_NEON_AUTH_URL || '').trim().replace(/\/+$/, '');

function getNeonAuthUrl(): string {
  if (!NEON_AUTH_URL) {
    throw new Error('auth_config_missing: EXPO_PUBLIC_NEON_AUTH_URL is empty');
  }
  return NEON_AUTH_URL;
}

function withAuthJwt(payload: any, res: Response): any {
  return {
    ...payload,
    __authJwt: res.headers.get('set-auth-jwt') || '',
  };
}

async function request(method: 'GET' | 'POST', path: string, body?: Record<string, unknown>): Promise<any> {
  const baseUrl = getNeonAuthUrl();
  let res: Response;

  try {
    res = await fetch(`${baseUrl}${path}`, {
      method,
      headers: body ? { 'Content-Type': 'application/json' } : undefined,
      credentials: 'include',
      body: body ? JSON.stringify(body) : undefined,
    });
  } catch {
    throw new Error('auth service unavailable');
  }

  const payload: any = await res.json().catch(() => ({}));
  const merged = withAuthJwt(payload, res);

  if (!res.ok) {
    const msg =
      merged?.error?.message ||
      merged?.message ||
      merged?.error ||
      `auth request failed (${res.status})`;
    throw new Error(`${msg} (status ${res.status})`);
  }

  return merged;
}

async function post(path: string, body: Record<string, unknown>): Promise<any> {
  return request('POST', path, body);
}

async function get(path: string): Promise<any> {
  return request('GET', path);
}

export const neonAuth = {
  async signUpWithEmail(email: string, password: string) {
    return post('/sign-up/email', {
      email,
      password,
      name: email.split('@')[0] || 'User',
    });
  },

  async signInWithEmail(email: string, password: string) {
    return post('/sign-in/email', { email, password });
  },

  async getSession() {
    return get('/get-session');
  },

  async getToken() {
    const response = await get('/token');
    return neonAuth.tokenFrom(response);
  },

  async signOut() {
    return;
  },

  tokenFrom(response: any): string {
    return (
      response?.__authJwt ||
      response?.data?.session?.token ||
      response?.session?.token ||
      response?.data?.session?.access_token ||
      response?.session?.access_token ||
      response?.data?.access_token ||
      response?.access_token ||
      response?.data?.token ||
      response?.token ||
      ''
    );
  },
};
