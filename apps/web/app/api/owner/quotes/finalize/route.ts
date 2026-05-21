import { NextResponse } from "next/server";
import { getDbPool } from "@/lib/db";
import { appendMilestone, calculateFinalQuote, isOwnerRequest, parseNumber } from "@/lib/owner-command-center";

export async function POST(req: Request) {
  const body = await req.json().catch(() => ({}));
  if (!isOwnerRequest(body.password ?? null)) return NextResponse.json({ error: "unauthorized" }, { status: 401 });
  const pool = getDbPool();
  const quoteRequestId = String(body.quote_request_id || "");
  if (!pool || !quoteRequestId) return NextResponse.json({ error: "missing quote_request_id" }, { status: 400 });
  const calc = calculateFinalQuote({
    supplier_machine_cost: parseNumber(body.supplier_machine_cost), supplier_ddp_total_cost: parseNumber(body.supplier_ddp_total_cost),
    tradepi_margin_type: body.tradepi_margin_type === "fixed" ? "fixed" : "percent", tradepi_margin_value: parseNumber(body.tradepi_margin_value), escrow_fee_estimate: parseNumber(body.escrow_fee_estimate),
  });
  await pool.query("insert into supplier_quote_inputs (quote_request_id,supplier_machine_cost,supplier_ddp_total_cost,production_days,shipping_days,customs_days,quote_valid_until,internal_notes) values ($1,$2,$3,$4,$5,$6,$7,$8)", [quoteRequestId, parseNumber(body.supplier_machine_cost), parseNumber(body.supplier_ddp_total_cost), parseNumber(body.production_days), parseNumber(body.shipping_days), parseNumber(body.customs_days), body.quote_valid_until ?? null, body.internal_notes ?? null]);
  await pool.query("insert into customer_final_quotes (quote_request_id,final_customer_price,supplier_total_cost,tradepi_margin,escrow_fee_estimate,quote_valid_until,terms_json) values ($1,$2,$3,$4,$5,$6,$7)", [quoteRequestId, calc.final_customer_quote, calc.supplier_total_cost, calc.tradepi_margin, calc.escrow_fee_paid_by_tradepi, body.quote_valid_until ?? null, JSON.stringify({ production_days: body.production_days, shipping_days: body.shipping_days, customs_days: body.customs_days, estimated_delivery_window_days: body.estimated_delivery_window_days ?? "75-80", ddp_confirmation_required: true })]);
  await appendMilestone(quoteRequestId, "customer quote prepared");
  return NextResponse.json({ ok: true, quote_request_id: quoteRequestId, public_quote: { final_customer_quote: calc.final_customer_quote, quote_valid_until: body.quote_valid_until ?? null } });
}
