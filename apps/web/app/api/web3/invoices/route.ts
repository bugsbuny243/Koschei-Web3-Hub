import { NextRequest, NextResponse } from "next/server";
import { isAddress } from "ethers";
import { web3Db } from "@/lib/web3-db";
import { createAccountingEntry } from "@/lib/web3-service";

export const runtime = "nodejs";
export async function GET() {
  const invoices = await web3Db.invoices.list();
  return NextResponse.json({ invoices });
}

export async function POST(req: NextRequest) {
  try {
    const body = await req.json();
    const requiredFields = ["chain_slug", "stablecoin_symbol", "stablecoin_contract", "receiver_address", "expected_amount"] as const;
    for (const field of requiredFields) {
      if (!body?.[field]?.toString().trim()) {
        return NextResponse.json({ ok: false, error: `${field} is required` }, { status: 400 });
      }
    }

    const expectedAmount = Number(body.expected_amount);
    if (!Number.isFinite(expectedAmount) || expectedAmount <= 0) {
      return NextResponse.json({ ok: false, error: "expected_amount must be a number greater than 0" }, { status: 400 });
    }

    const chain = await web3Db.chains.bySlug(body.chain_slug);
    if (!chain) return NextResponse.json({ ok: false, error: "Invalid chain_slug" }, { status: 400 });
    if (!isAddress(body.stablecoin_contract ?? "")) return NextResponse.json({ ok: false, error: "Invalid stablecoin_contract" }, { status: 400 });
    if (!isAddress(body.receiver_address ?? "")) return NextResponse.json({ ok: false, error: "Invalid receiver_address" }, { status: 400 });

    const normalizedDueAt = body.due_at === "" || body.due_at === undefined || body.due_at === null
      ? null
      : body.due_at;

    const invoice = await web3Db.invoices.create({
      chain_slug: body.chain_slug,
      stablecoin_symbol: body.stablecoin_symbol,
      stablecoin_contract: body.stablecoin_contract,
      receiver_address: body.receiver_address,
      expected_amount: expectedAmount.toString(),
      currency: body.currency ?? "USD",
      due_at: normalizedDueAt,
      metadata: body.metadata ?? {}
    });

    if (!invoice) return NextResponse.json({ ok: false, error: "Failed to create invoice" }, { status: 500 });
    await createAccountingEntry({
      invoice_id: invoice.id,
      payment_event_id: null,
      entry_type: "invoice_created",
      amount: invoice.expected_amount,
      currency: invoice.currency,
      description: "Invoice created",
      metadata: {
        chain_slug: invoice.chain_slug,
        stablecoin_symbol: invoice.stablecoin_symbol,
        stablecoin_contract: invoice.stablecoin_contract,
        receiver_address: invoice.receiver_address,
        expected_amount: invoice.expected_amount,
        currency: invoice.currency,
        due_at: invoice.due_at,
        metadata: invoice.metadata ?? {}
      }
    });
    return NextResponse.json({ ok: true, invoice }, { status: 201 });
  } catch (error) {
    const message = error instanceof Error ? error.message : "Failed to create invoice";
    return NextResponse.json({ ok: false, error: message }, { status: 500 });
  }
}
