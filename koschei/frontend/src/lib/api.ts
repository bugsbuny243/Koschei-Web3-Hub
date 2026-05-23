const API_BASE = import.meta.env.VITE_API_BASE_URL as string | undefined;

export const apiConnected = Boolean(API_BASE);

async function request(path: string, init?: RequestInit) {
  if (!API_BASE) throw new Error('API not connected yet');
  const res = await fetch(`${API_BASE}${path}`, {
    headers: { 'Content-Type': 'application/json', ...(init?.headers || {}) },
    ...init,
  });
  const data = await res.json().catch(() => ({}));
  if (!res.ok) throw new Error(data.error || 'Request failed');
  return data;
}

export const api = {
  getPlans: () => request('/api/plans'),
  createPaymentRequest: (body: unknown) => request('/api/billing/manual-payment-request', { method: 'POST', body: JSON.stringify(body) }),
  getCredits: (email: string) => request(`/api/credits?email=${encodeURIComponent(email)}`),
  getJobs: (email: string) => request(`/api/jobs?email=${encodeURIComponent(email)}`),
  createJob: (body: unknown) => request('/api/jobs', { method: 'POST', body: JSON.stringify(body) }),
  ownerPaymentRequests: (password: string) => request('/api/owner/payment-requests', { headers: { 'x-admin-password': password } }),
  ownerActivatePlan: (password: string, body: unknown) => request('/api/owner/activate-plan', { method: 'POST', headers: { 'x-admin-password': password }, body: JSON.stringify(body) }),
  ownerGrantCredits: (password: string, body: unknown) => request('/api/owner/grant-credits', { method: 'POST', headers: { 'x-admin-password': password }, body: JSON.stringify(body) }),
  ownerUpdateJobStatus: (password: string, id: string, body: unknown) => request(`/api/owner/jobs/${id}/status`, { method: 'PATCH', headers: { 'x-admin-password': password }, body: JSON.stringify(body) }),
};
