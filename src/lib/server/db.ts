import "server-only";
import { query } from "@/lib/server/neon";

export type Product = {
  id: string;
  name: string;
  price_try: number;
  monthly_credits: number;
  pack_type: string;
  output_quota: number;
  shopier_url: string;
  is_active: boolean;
};
export type ManualOrderInput = { email: string; fullName: string; productId: string; rawPayload?: Record<string, unknown> };
export type OutputInput = { entitlementId: string; projectId?: string; outputType: "game_asset" | "risk_report" | "launch_copy" | "docs_bundle"; title?: string; ecosystem?: string; metadata?: Record<string, unknown>; contentJson?: Record<string, unknown>; contentText?: string; usedAi?: boolean; usedFallback?: boolean };
export type PaymentRequestStatus = "pending" | "manual_verified" | "paid" | "rejected";

export async function getProducts() {
  return query<Product>(`SELECT id, name, price_try, monthly_credits, pack_type, output_quota, shopier_url, is_active
FROM plans
WHERE is_active = true
ORDER BY price_try ASC`);
}

export async function createManualOrder(input: ManualOrderInput) {
  const rows = await query(`INSERT INTO payment_requests (email, full_name, plan, product_id, payment_provider, payment_reference, amount_try, currency, status, raw_payload)
SELECT $1, $2, name, id, 'manual', gen_random_uuid()::text, price_try, 'TRY', 'pending', $4::jsonb
FROM plans
WHERE id = $3 AND is_active = true
RETURNING *`, [input.email.toLowerCase(), input.fullName, input.productId, JSON.stringify(input.rawPayload || {})]);
  if (!rows[0]) throw new Error("Active plan not found.");
  return rows[0];
}

export async function reviewPaymentRequest(paymentRequestId: string, status: PaymentRequestStatus) {
  const rows = await query(`WITH reviewed AS (
  UPDATE payment_requests
  SET status = $2, reviewed_at = now()
  WHERE id = $1 AND status = 'pending'
  RETURNING *
), entitlement AS (
  INSERT INTO entitlements (email, payment_request_id, plan_id, outputs_total, outputs_remaining, status)
  SELECT reviewed.email, reviewed.id, plans.id, plans.output_quota, plans.output_quota, 'active'
  FROM reviewed
  JOIN plans ON plans.id = reviewed.product_id
  WHERE reviewed.status IN ('paid', 'manual_verified')
  ON CONFLICT (payment_request_id) DO UPDATE SET updated_at = now()
  RETURNING id
)
SELECT reviewed.*, (SELECT id FROM entitlement) AS entitlement_id
FROM reviewed`, [paymentRequestId, status]);
  if (!rows[0]) throw new Error("Pending payment request not found.");
  return rows[0];
}

export async function decrementEntitlementOutput(entitlementId: string) {
  const rows = await query(`UPDATE entitlements
SET outputs_remaining = outputs_remaining - 1,
    status = CASE WHEN outputs_remaining = 1 THEN 'exhausted' ELSE status END,
    updated_at = now()
WHERE id = $1 AND status = 'active' AND outputs_remaining > 0
RETURNING *`, [entitlementId]);
  if (!rows[0]) throw new Error("No active output rights remain.");
  return rows[0];
}

export async function saveGeneratedOutput(input: OutputInput) {
  const contentJson = { ...(input.metadata || {}), ...(input.contentJson || {}) };
  const rows = await query(`WITH spent AS (
  UPDATE entitlements
  SET outputs_remaining = outputs_remaining - 1,
      status = CASE WHEN outputs_remaining = 1 THEN 'exhausted' ELSE status END,
      updated_at = now()
  WHERE id = $1 AND status = 'active' AND outputs_remaining > 0
  RETURNING id, email
)
INSERT INTO web3_outputs (email, entitlement_id, project_id, output_type, title, ecosystem, content_json, content_text, used_ai, used_fallback)
SELECT email, id, $2, $3, $4, $5, $6::jsonb, $7, $8, $9 FROM spent
RETURNING *`, [input.entitlementId, input.projectId || null, input.outputType, input.title || null, input.ecosystem || null, JSON.stringify(contentJson), input.contentText || null, input.usedAi || false, input.usedFallback || false]);
  if (!rows[0]) throw new Error("No active output rights remain.");
  return rows[0];
}

export async function listAdminPlans() {
  return query<Product>(`SELECT id, name, price_try, monthly_credits, pack_type, output_quota, shopier_url, is_active
FROM plans
ORDER BY price_try ASC`);
}

export async function listAdminPaymentRequests() {
  return query(`SELECT id, email, full_name, plan, product_id, payment_provider, payment_reference, amount_try, currency, status, created_at, reviewed_at
FROM payment_requests
ORDER BY created_at DESC
LIMIT 200`);
}

export async function listAdminEntitlements() {
  return query(`SELECT entitlements.id, entitlements.email, plans.name AS plan_name, entitlements.payment_request_id, entitlements.outputs_total, entitlements.outputs_remaining, entitlements.status, entitlements.created_at
FROM entitlements
JOIN plans ON entitlements.plan_id = plans.id
ORDER BY entitlements.created_at DESC
LIMIT 200`);
}

export async function listAdminOutputs() {
  return query(`SELECT web3_outputs.id, web3_outputs.email, web3_outputs.output_type, web3_outputs.title, web3_outputs.ecosystem, web3_outputs.used_ai, web3_outputs.used_fallback, entitlements.outputs_remaining, web3_outputs.created_at
FROM web3_outputs
JOIN entitlements ON web3_outputs.entitlement_id = entitlements.id
ORDER BY web3_outputs.created_at DESC
LIMIT 200`);
}

export type UserDashboard = {
  email: string;
  plan_name: string | null;
  package_status: string | null;
  outputs_remaining: number;
  saved_outputs: number;
};

export async function createMemberAccount(email: string, passwordHash: string) {
  const rows = await query<{ email: string }>(`INSERT INTO member_accounts (email, password_hash, role, status)
VALUES (lower($1), $2, 'user', 'active')
ON CONFLICT DO NOTHING
RETURNING email`, [email, passwordHash]);
  if (!rows[0]) throw new Error("Account already exists. Please sign in.");
  return rows[0];
}

export async function getMemberAccountForLogin(email: string) {
  const rows = await query<{ email: string; password_hash: string }>(`SELECT email, password_hash
FROM member_accounts
WHERE lower(email) = lower($1) AND role = 'user' AND status = 'active'
LIMIT 1`, [email]);
  return rows[0];
}

export async function getUserDashboard(email: string) {
  const rows = await query<UserDashboard>(`SELECT account.email,
  latest.plan_name,
  latest.status AS package_status,
  COALESCE(rights.outputs_remaining, 0)::int AS outputs_remaining,
  COALESCE(outputs.saved_outputs, 0)::int AS saved_outputs
FROM member_accounts account
LEFT JOIN LATERAL (
  SELECT plans.name AS plan_name, entitlements.status
  FROM entitlements
  JOIN plans ON plans.id = entitlements.plan_id
  WHERE lower(entitlements.email) = lower(account.email)
  ORDER BY (entitlements.status = 'active') DESC, entitlements.created_at DESC
  LIMIT 1
) latest ON true
LEFT JOIN LATERAL (
  SELECT SUM(entitlements.outputs_remaining) AS outputs_remaining
  FROM entitlements
  WHERE lower(entitlements.email) = lower(account.email) AND entitlements.status = 'active'
) rights ON true
LEFT JOIN LATERAL (
  SELECT COUNT(*) AS saved_outputs
  FROM web3_outputs
  WHERE lower(web3_outputs.email) = lower(account.email)
) outputs ON true
WHERE lower(account.email) = lower($1) AND account.role = 'user' AND account.status = 'active'
LIMIT 1`, [email]);
  return rows[0];
}
