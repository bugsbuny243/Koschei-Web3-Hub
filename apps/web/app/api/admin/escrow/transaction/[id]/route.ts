import { NextRequest, NextResponse } from "next/server";
import { fetchEscrowTransaction } from "@/lib/escrow-client";
import { escrowTransactions } from "@/lib/payment-store";
import { isAdminAuthed } from "@/lib/admin-auth";

export async function GET(request: NextRequest, { params }: { params: Promise<{ id: string }> }) {
  const password = request.headers.get("x-admin-password");
  if (!isAdminAuthed(password)) return NextResponse.json({ error: "Unauthorized" }, { status: 401 });

  const { id } = await params;
  const latest = await fetchEscrowTransaction(id);
  const existing = escrowTransactions.find((x) => x.escrow_transaction_id === id);
  if (existing) {
    existing.escrow_status = latest.status ?? existing.escrow_status;
    existing.raw_response = latest;
    existing.updated_at = new Date().toISOString();
  }

  return NextResponse.json({ transaction: latest, local: existing ?? null });
}
