import { getDbPool } from "@/lib/db";
export default async function SupplierPaymentsPage({ params, searchParams }: { params: Promise<{ id: string }>; searchParams: Promise<{ password?: string }> }) {
  const { id } = await params; const { password } = await searchParams;
  if (!process.env.ADMIN_PASSWORD || password !== process.env.ADMIN_PASSWORD) return <div className="page-stack"><h1>Admin Access Required</h1></div>;
  const pool = getDbPool(); if (!pool) return <div className="page-stack">DB unavailable</div>;
  const q=(await pool.query("select * from customer_quotes where quote_request_id=$1 order by created_at desc limit 1",[id])).rows[0];
  const rows=(await pool.query("select * from supplier_payments where quote_request_id=$1 order by created_at asc",[id])).rows;
  const total=Number(q?.internal_total_cost??0), d=total*0.3, b=total*0.7;
  return <div className="page-stack"><h1>Supplier Payments</h1><p>supplier landed cost: {total}</p><p>30% deposit expected: {d.toFixed(2)}</p><p>70% balance expected: {b.toFixed(2)}</p>{rows.map((r:any)=><div key={r.id}><p>{r.payment_stage}: {r.status} / proof_file_url: {r.proof_file_url||'N/A'} / private notes: {r.private_notes||'N/A'}</p></div>)}</div>;
}
