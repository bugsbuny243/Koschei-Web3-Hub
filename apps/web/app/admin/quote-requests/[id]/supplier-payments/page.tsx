import { getDbPool } from "@/lib/db";

export default async function SupplierPaymentsPage({ params, searchParams }: { params: Promise<{ id: string }>; searchParams: Promise<{ password?: string }> }) {
  const { id } = await params;
  const { password } = await searchParams;
  if (!process.env.ADMIN_PASSWORD || password !== process.env.ADMIN_PASSWORD) return <div className="page-stack"><h1>Admin Access Required</h1></div>;

  const pool = getDbPool();
  if (!pool) return <div className="page-stack">DB unavailable</div>;

  const supplier = (await pool.query("select * from supplier_ddp_quotes where quote_request_id=$1 order by created_at desc limit 1", [id])).rows[0];
  const escrow = (await pool.query("select * from escrow_transactions where quote_request_id=$1 order by created_at desc limit 1", [id])).rows[0];
  const rows = (await pool.query("select * from supplier_payment_milestones where quote_request_id=$1 order by created_at asc", [id])).rows;

  const total = Number(supplier?.raw_ddp_cost ?? 0);
  const deposit = total * 0.3;
  const balance = total * 0.7;
  const escrowSecured = ["funded", "completed", "payment_secured"].includes(String(escrow?.escrow_status ?? ""));

  return <div className="page-stack"><h1>Supplier Payments (Internal)</h1><p>Escrow status: {escrow?.escrow_status ?? "missing"}</p><p>Supplier DDP total: {total.toFixed(2)}</p><p>30% production deposit: {deposit.toFixed(2)}</p><p>70% before shipment/delivery release: {balance.toFixed(2)}</p>{rows.map((r: any) => <div key={r.id}><p>{r.milestone_name}: {r.status} / amount: {r.milestone_amount}</p></div>)}<form className="card" action={`/api/admin/quote-requests/${id}/supplier-payment-milestones?password=${password}`} method="post"><input type="number" name="supplier_total_amount" min="0" step="0.01" defaultValue={total || undefined} className="input" placeholder="supplier_total_amount" required/><input name="deposit_status" className="input" placeholder="deposit_status (pending/paid)" /><input name="balance_status" className="input" placeholder="balance_status (pending/paid)" /><textarea name="deposit_private_notes" className="input" placeholder="deposit_private_notes" /><textarea name="balance_private_notes" className="input" placeholder="balance_private_notes" /><button className="btn" type="submit" disabled={!escrowSecured}>Record supplier milestones</button>{!escrowSecured && <p>Customer escrow payment must be secured first.</p>}</form></div>;
}
