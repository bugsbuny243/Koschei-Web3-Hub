import { NextRequest, NextResponse } from "next/server";
import { web3Db } from "@/lib/web3-db";

export async function GET(_req: NextRequest, { params }: { params: { id: string } }) {
  const invoiceResult = await web3Db.query(`select * from web3_invoices where id = $1`, [params.id]);
  if (invoiceResult.rows.length === 0) return NextResponse.json({ error: "Not found" }, { status: 404 });
  const paymentEvents = await web3Db.query(`select * from web3_payment_events where invoice_id = $1 order by created_at desc`, [params.id]);
  const entries = await web3Db.query(`select * from web3_accounting_entries where invoice_id = $1 order by created_at desc`, [params.id]);
  return NextResponse.json({ invoice: invoiceResult.rows[0], payment_events: paymentEvents.rows, accounting_entries: entries.rows });
}
