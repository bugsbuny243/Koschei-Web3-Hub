import { auth } from './auth';

const API_BASE = process.env.EXPO_PUBLIC_API_URL?.trim() || '';

type ReqOptions = RequestInit & { authRequired?: boolean };

async function request(path: string, options: ReqOptions = {}) {
  const headers: Record<string, string> = { 'Content-Type': 'application/json' };
  if (options.authRequired) {
    const token = await auth.getToken();
    if (!token) throw new Error('You are not logged in.');
    headers.Authorization = `Bearer ${token}`;
  }
  const res = await fetch(`${API_BASE}${path}`, { ...options, headers: { ...headers, ...(options.headers as Record<string,string> || {}) } });
  const data = await res.json().catch(() => ({}));
  if (!res.ok) throw new Error(data.error || `Request failed (${res.status})`);
  return data;
}

export const api = {
  getPlans: () => request('/api/plans'),
  getVersion: () => request('/api/version'),
  register: (email: string, password: string) => request('/api/auth/register', { method: 'POST', body: JSON.stringify({ email, password }) }),
  login: (email: string, password: string) => request('/api/auth/login', { method: 'POST', body: JSON.stringify({ email, password }) }),
  credits: (email: string) => request(`/api/credits?email=${encodeURIComponent(email)}`, { authRequired: true }),
  chat: (payload: unknown) => request('/api/runtime/route', { method: 'POST', body: JSON.stringify(payload), authRequired: true }),
};
