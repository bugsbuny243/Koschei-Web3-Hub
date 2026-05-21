import { NextResponse } from "next/server";
import { calculateFinalQuote, parseNumber } from "@/lib/owner-command-center";
import { isOwnerAuthenticated } from "@/lib/owner-auth";

export async function POST(req: Request) {
  const body = await req.json().catch(() => ({}));
  if (!(await isOwnerAuthenticated(body.password ?? null))) return NextResponse.json({ error: "unauthorized" }, { status: 401 });
  const result = calculateFinalQuote({
    supplier_machine_cost: parseNumber(body.supplier_machine_cost),
    supplier_ddp_total_cost: parseNumber(body.supplier_ddp_total_cost),
    tradepi_margin_type: body.tradepi_margin_type === "fixed" ? "fixed" : "percent",
    tradepi_margin_value: parseNumber(body.tradepi_margin_value),
    escrow_fee_estimate: parseNumber(body.escrow_fee_estimate),
  });
  return NextResponse.json({ ok: true, ...result, estimated_delivery_window_days: body.estimated_delivery_window_days ?? "75-80", note: "Final DDP requires supplier confirmation." });
}
