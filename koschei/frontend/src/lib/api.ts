import AsyncStorage from '@react-native-async-storage/async-storage';

const API_BASE = (process.env.EXPO_PUBLIC_API_URL || '').trim();
const TOKEN_KEY = 'koschei_token';

async function request(path: string, init?: RequestInit, auth = false) {
  const target = API_BASE ? `${API_BASE}${path}` : path;
  const headers: Record<string, string> = { 'Content-Type': 'application/json', ...(init?.headers as Record<string, string> || {}) };
  if (auth) {
    const token = await AsyncStorage.getItem(TOKEN_KEY);
    if (token) headers.Authorization = `Bearer ${token}`;
  }
  const res = await fetch(target, { ...init, headers });
  const data = await res.json().catch(() => ({}));
  if (!res.ok) {
    const base = data?.error || `Request failed (${res.status})`;
    const detail = typeof data?.detail === 'string' && data.detail.trim() ? `: ${data.detail}` : '';
    const err: any = new Error(`${base}${detail}`);
    err.status = res.status;
    err.data = data;
    throw err;
  }
  return data;
}

export const api = {
  get: (path: string) => request(path, undefined, true),
  me: () => request('/api/me', undefined, true),
  getPlans: () => request('/api/plans'),
  createPaymentRequest: (body: unknown) => request('/api/billing/manual-payment-request', { method: 'POST', body: JSON.stringify(body) }, true),
  getCredits: () => request('/api/credits/me', undefined, true),
  getRuntimeProjects: () => request('/api/runtime/projects', undefined, true),
  getRuntimeTasks: () => request('/api/runtime/tasks', undefined, true),
  getRuntimeLogs: (projectId: string) => request(`/api/runtime/logs/${projectId}`, undefined, true),
  createRuntimeProject: (body: unknown) => request('/api/runtime/projects', { method: 'POST', body: JSON.stringify(body) }, true),
};
