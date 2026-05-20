import { NextRequest, NextResponse } from "next/server";
import { createEscrowTransaction } from "@/lib/escrow-client";
import { customerQuotes, escrowTransactions } from "@/lib/payment-store";
import { isAdminAuthed } from "@/lib/admin-auth";

export async function POST(request: NextRequest) {
  const password = request.headers.get("x-admin-password");
  if (!isAdminAuthed(password)) return NextResponse.json({ error: "Unauthorized" }, { status: 401 });

  const body = await request.json();
  const quote = customerQuotes.find((q) => q.id === body.customer_quote_id);
  if (!quote) return NextResponse.json({ error: "Quote not found" }, { status: 404 });
  if (!["approved_internal", "accepted_by_customer"].includes(quote.status)) {
    return NextResponse.json({ error: "Quote status is not eligible" }, { status: 400 });
  }

  const { payload, data } = await createEscrowTransaction({
    buyerEmail: quote.buyerEmail,
    finalCustomerPrice: quote.finalCustomerPrice,
    itemTitle: quote.itemTitle,
    itemDescription: quote.itemDescription,
    feePayer: body.fee_payer ?? "buyer"
  });

  const record = {
    customer_quote_id: quote.id,
    quote_request_id: quote.quoteRequestId,
    escrow_transaction_id: data.id ?? data?.transaction_id,
    escrow_status: data.status ?? "draft",
    final_customer_price: quote.finalCustomerPrice,
    buyer_email: quote.buyerEmail,
    payment_link: data?.payment_url ?? null,
    raw_create_payload: payload,
    raw_response: data,
    created_at: new Date().toISOString(),
    supplier_deposit_status: "pending",
    supplier_balance_status: "pending"
  };
  escrowTransactions.push(record);

  return NextResponse.json(record);
}
