const API_BASE = (import.meta.env.EXPO_PUBLIC_API_URL as string | undefined)?.trim() || '';
const TOKEN_KEY = 'koschei_token';

export const tokenStore = {
  get: () => localStorage.getItem(TOKEN_KEY) || '',
  set: (token: string) => localStorage.setItem(TOKEN_KEY, token),
  clear: () => localStorage.removeItem(TOKEN_KEY),
};

async function request(path: string, init?: RequestInit, auth = false) {
  const target = API_BASE ? `${API_BASE}${path}` : path;
  const headers: Record<string, string> = { 'Content-Type': 'application/json', ...(init?.headers as Record<string, string> || {}) };
  if (auth && tokenStore.get()) headers.Authorization = `Bearer ${tokenStore.get()}`;
  const res = await fetch(target, { ...init, headers });
  const data = await res.json().catch(() => ({}));
  if (!res.ok) throw new Error(data.error || `Request failed (${res.status})`);
  return data;
}

export const api = {
  register: (body: {email:string;password:string}) => request('/api/auth/register', { method:'POST', body: JSON.stringify(body)}),
  login: (body: {email:string;password:string}) => request('/api/auth/login', { method:'POST', body: JSON.stringify(body)}),
  me: () => request('/api/me', undefined, true),
  getPlans: () => request('/api/plans'),
  createPaymentRequest: (body: unknown) => request('/api/billing/manual-payment-request', { method: 'POST', body: JSON.stringify(body) }, true),
  getRuntimeProjects: (email: string) => request(`/api/runtime/projects?email=${encodeURIComponent(email)}`, undefined, true),
  getRuntimeTasks: (email: string) => request(`/api/runtime/tasks?email=${encodeURIComponent(email)}`, undefined, true),
  getRuntimeLogs: (projectId: string) => request(`/api/runtime/logs/${projectId}`, undefined, true),
  createRuntimeProject: (body: unknown) => request('/api/runtime/projects', { method: 'POST', body: JSON.stringify(body) }, true),
};
