import { NextRequest, NextResponse } from "next/server";
import { web3Db } from "@/lib/web3-db";
import { createAccountingEntry } from "@/lib/web3-service";

export async function GET() {
  const invoices = await web3Db.invoices.list();
  return NextResponse.json({ invoices });
}

export async function POST(req: NextRequest) {
  const body = await req.json();
  const chain = await web3Db.chains.bySlug(body.chain_slug);
  if (!chain) return NextResponse.json({ error: "Invalid chain_slug" }, { status: 400 });

  const invoice = await web3Db.invoices.create({
    chain_id: chain.id,
    stablecoin_symbol: body.stablecoin_symbol,
    stablecoin_contract: body.stablecoin_contract,
    receiver_address: body.receiver_address,
    expected_amount: body.expected_amount,
    currency: body.currency ?? "USD",
    due_at: body.due_at ?? null,
    metadata: body.metadata ?? {}
  });

  if (!invoice) return NextResponse.json({ error: "Failed to create invoice" }, { status: 500 });
  await createAccountingEntry(invoice.id, null, "invoice_created", { invoice_id: invoice.id });
  return NextResponse.json({ invoice }, { status: 201 });
}
