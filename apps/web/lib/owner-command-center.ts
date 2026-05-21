import "server-only";

import { getDbPool } from "@/lib/db";

export function parseNumber(value: unknown): number {
  const n = Number(value ?? 0);
  return Number.isFinite(n) ? n : 0;
}

export function isOwnerRequest(password: string | null): boolean {
  return !!process.env.ADMIN_PASSWORD && password === process.env.ADMIN_PASSWORD;
}

export function calculateFinalQuote(input: {
  supplier_machine_cost: number;
  supplier_ddp_total_cost: number;
  tradepi_margin_type: "percent" | "fixed";
  tradepi_margin_value: number;
  escrow_fee_estimate: number;
}) {
  const supplierTotalCost = parseNumber(input.supplier_ddp_total_cost || input.supplier_machine_cost);
  const margin = input.tradepi_margin_type === "percent"
    ? (supplierTotalCost * parseNumber(input.tradepi_margin_value)) / 100
    : parseNumber(input.tradepi_margin_value);
  const escrowFee = parseNumber(input.escrow_fee_estimate);
  const finalCustomerQuote = supplierTotalCost + margin + escrowFee;

  return {
    supplier_total_cost: supplierTotalCost,
    tradepi_margin: margin,
    escrow_fee_paid_by_tradepi: escrowFee,
    final_customer_quote: finalCustomerQuote,
  };
}

export async function appendMilestone(quoteRequestId: string, name: string, status = "completed") {
  const pool = getDbPool();
  if (!pool) return;
  await pool.query(
    "insert into operation_milestones (quote_request_id,milestone_name,status,completed_at) values ($1,$2,$3,case when $3='completed' then now() else null end)",
    [quoteRequestId, name, status],
  );
}
