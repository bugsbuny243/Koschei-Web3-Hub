import { getDbPool } from "@/lib/db";

export default async function EscrowQuotePage({ params, searchParams }: { params: Promise<{ id: string }>; searchParams: Promise<{ password?: string }> }) {
  const { id } = await params;
  const { password } = await searchParams;
  if (!process.env.ADMIN_PASSWORD || password !== process.env.ADMIN_PASSWORD) return <div className="page-stack"><h1>Admin Access Required</h1></div>;

  const pool = getDbPool();
  if (!pool) return <div className="page-stack">DB unavailable</div>;

  const quote = (await pool.query("select cfq.*, qr.company_name, qr.full_name, qr.email from customer_final_quotes cfq join quote_requests qr on qr.id=cfq.quote_request_id where cfq.quote_request_id=$1 order by cfq.created_at desc limit 1", [id])).rows[0];
  const escrow = (await pool.query("select * from escrow_transactions where quote_request_id=$1 order by created_at desc limit 1", [id])).rows[0];

  return <div className="page-stack"><h1>Escrow Setup</h1>{quote ? <><p>Quote request id: {id}</p><p>Customer/company: {quote.full_name} / {quote.company_name}</p><p>Buyer email: {quote.email}</p><p>Final customer price: {quote.final_customer_price}</p><p>Escrow fee payer: TradePi internal margin buffer</p><form action={`/api/admin/quote-requests/${id}/escrow/create-transaction?password=${password}`} method="post"><button className="btn btn-primary" type="submit">Create Escrow Transaction</button></form></> : <p>No final customer quote found.</p>}{escrow && <><p>escrow_transaction_id: {escrow.escrow_transaction_id}</p><p>escrow_status: {escrow.escrow_status}</p><p>payment_link: {escrow.payment_link ?? "N/A"}</p></>}</div>;
}
