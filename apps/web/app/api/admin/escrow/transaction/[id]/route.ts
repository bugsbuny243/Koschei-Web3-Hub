import { NextRequest, NextResponse } from "next/server";
import { fetchEscrowTransaction } from "@/lib/escrow-client";
import { isAdminAuthed } from "@/lib/admin-auth";
import { getDbPool } from "@/lib/db";

export async function GET(request: NextRequest, { params }: { params: Promise<{ id: string }> }) {
  const password = request.headers.get("x-admin-password");
  if (!isAdminAuthed(password)) return NextResponse.json({ error: "Unauthorized" }, { status: 401 });
  const { id } = await params;
  const latest = await fetchEscrowTransaction(id);
  const pool = getDbPool();
  let local = null;
  if (pool) {
    await pool.query("update escrow_transactions set escrow_status=$2, raw_response=$3, updated_at=now() where escrow_transaction_id=$1", [id, latest.status ?? null, JSON.stringify(latest)]);
    local = (await pool.query("select * from escrow_transactions where escrow_transaction_id=$1", [id])).rows[0] ?? null;
  }
  return NextResponse.json({ transaction: latest, local });
}
