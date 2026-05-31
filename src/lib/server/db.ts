import "server-only";
import { query } from "@/lib/server/neon";

export type Product = { id: string; slug: string; name: string; price_try_cents: number; output_quota: number; shopier_url: string; is_active: boolean };
export type CustomerInput = { email: string; fullName: string; company?: string; country?: string; source?: string };
export type OutputInput = { entitlementId: string; projectId?: string; outputType: "game_asset" | "risk_report" | "launch_copy" | "docs_bundle"; metadata?: Record<string, unknown>; contentJson?: Record<string, unknown>; contentText?: string; usedAi?: boolean; usedFallback?: boolean };

export async function getProducts() {
  return query<Product>(`SELECT id, slug, name, price_try_cents::int, output_quota::int, shopier_url, is_active FROM products WHERE is_active = true ORDER BY price_try_cents`);
}
export async function createCustomer(input: CustomerInput) {
  const rows = await query<{ id: string }>(`INSERT INTO customers (email, full_name, company, country, source) VALUES ($1, $2, $3, $4, $5) ON CONFLICT (email) DO UPDATE SET full_name = EXCLUDED.full_name, company = COALESCE(EXCLUDED.company, customers.company), country = COALESCE(EXCLUDED.country, customers.country), source = COALESCE(EXCLUDED.source, customers.source), updated_at = now() RETURNING id`, [input.email.toLowerCase(), input.fullName, input.company || null, input.country || null, input.source || null]);
  return rows[0];
}
export async function createManualOrder(customerId: string, productId: string, rawPayload: Record<string, unknown> = {}) {
  const rows = await query(`INSERT INTO orders (customer_id, product_id, provider, provider_order_id, amount_try_cents, status, raw_payload) SELECT $1, id, 'manual', gen_random_uuid()::text, price_try_cents, 'pending', $3::jsonb FROM products WHERE id = $2 AND is_active = true RETURNING *`, [customerId, productId, JSON.stringify(rawPayload)]);
  if (!rows[0]) throw new Error("Active product not found.");
  return rows[0];
}
export async function createEntitlementFromPaidOrder(orderId: string) {
  const rows = await query(`INSERT INTO entitlements (customer_id, order_id, product_id, outputs_total, outputs_remaining, status) SELECT o.customer_id, o.id, o.product_id, p.output_quota, p.output_quota, 'active' FROM orders o JOIN products p ON p.id = o.product_id WHERE o.id = $1 AND o.status IN ('paid', 'manual_verified') ON CONFLICT (order_id) DO UPDATE SET updated_at = now() RETURNING *`, [orderId]);
  if (!rows[0]) throw new Error("Only verified paid orders can create entitlements.");
  return rows[0];
}
export async function decrementEntitlementOutput(entitlementId: string) {
  const rows = await query(`UPDATE entitlements SET outputs_remaining = outputs_remaining - 1, status = CASE WHEN outputs_remaining = 1 THEN 'exhausted'::entitlement_status ELSE status END, updated_at = now() WHERE id = $1 AND status = 'active' AND outputs_remaining > 0 RETURNING *`, [entitlementId]);
  if (!rows[0]) throw new Error("No active output rights remain.");
  return rows[0];
}
export async function saveGeneratedOutput(input: OutputInput) {
  const rows = await query(`WITH spent AS (UPDATE entitlements SET outputs_remaining = outputs_remaining - 1, status = CASE WHEN outputs_remaining = 1 THEN 'exhausted'::entitlement_status ELSE status END, updated_at = now() WHERE id = $1 AND status = 'active' AND outputs_remaining > 0 RETURNING id, customer_id) INSERT INTO generated_outputs (customer_id, entitlement_id, project_id, output_type, metadata, content_json, content_text, used_ai, used_fallback) SELECT customer_id, id, $2, $3, $4::jsonb, $5::jsonb, $6, $7, $8 FROM spent RETURNING *`, [input.entitlementId, input.projectId || null, input.outputType, JSON.stringify(input.metadata || {}), input.contentJson ? JSON.stringify(input.contentJson) : null, input.contentText || null, input.usedAi || false, input.usedFallback || false]);
  if (!rows[0]) throw new Error("No active output rights remain.");
  return rows[0];
}
export async function listAdminOrders() { return query(`SELECT o.*, c.email AS customer_email, c.full_name AS customer_name, p.name AS product_name FROM orders o JOIN customers c ON c.id = o.customer_id JOIN products p ON p.id = o.product_id ORDER BY o.created_at DESC LIMIT 200`); }
export async function listAdminEntitlements() { return query(`SELECT e.*, c.email AS customer_email, p.name AS product_name FROM entitlements e JOIN customers c ON c.id = e.customer_id JOIN products p ON p.id = e.product_id ORDER BY e.created_at DESC LIMIT 200`); }
export async function listAdminOutputs() { return query(`SELECT g.*, c.email AS customer_email, e.outputs_remaining::int FROM generated_outputs g JOIN customers c ON c.id = g.customer_id JOIN entitlements e ON e.id = g.entitlement_id ORDER BY g.created_at DESC LIMIT 200`); }
