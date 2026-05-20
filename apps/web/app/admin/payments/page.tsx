import { escrowTransactions } from "@/lib/payment-store";

export default async function AdminPaymentsPage({ searchParams }: { searchParams: Promise<{ password?: string }> }) {
  const { password } = await searchParams;
  if (!process.env.ADMIN_PASSWORD || password !== process.env.ADMIN_PASSWORD) return <div className="page-stack"><h1>Admin Access Required</h1></div>;

  return (
    <div className="page-stack">
      <h1>Admin Payments</h1>
      <p>Escrow.com payments are created only after quote approval. No public checkout is enabled.</p>
      <table>
        <thead><tr><th>Quote Request</th><th>Customer</th><th>Quote Amount</th><th>Escrow Txn</th><th>Escrow Status</th><th>Supplier 30%</th><th>Supplier 70%</th><th>Gross Profit</th><th>Created</th></tr></thead>
        <tbody>
          {escrowTransactions.map((row) => (
            <tr key={row.escrow_transaction_id}>
              <td>{row.quote_request_id}</td><td>{row.buyer_email}</td><td>{row.final_customer_price}</td><td>{row.escrow_transaction_id}</td><td>{row.escrow_status}</td><td>{row.supplier_deposit_status}</td><td>{row.supplier_balance_status}</td><td>{(row.final_customer_price - 0).toFixed(2)}</td><td>{row.created_at}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}
