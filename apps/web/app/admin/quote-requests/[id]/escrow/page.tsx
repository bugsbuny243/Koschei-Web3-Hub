import { customerQuotes, escrowTransactions } from "@/lib/payment-store";

export default async function EscrowQuotePage({ params, searchParams }: { params: Promise<{ id: string }>; searchParams: Promise<{ password?: string }> }) {
  const { id } = await params;
  const { password } = await searchParams;
  if (!process.env.ADMIN_PASSWORD || password !== process.env.ADMIN_PASSWORD) return <div className="page-stack"><h1>Admin Access Required</h1></div>;

  const quote = customerQuotes.find((q) => q.quoteRequestId === id);
  const escrow = escrowTransactions.find((e) => e.quote_request_id === id);

  return <div className="page-stack"><h1>Escrow Setup</h1>{quote ? <><p>{quote.itemTitle}</p><p>Final customer price: {quote.finalCustomerPrice}</p><p>Buyer email: {quote.buyerEmail}</p><p>{quote.itemDescription}</p><p>Escrow fee payer: buyer/seller/split/manual note</p><p>Create via POST /api/admin/escrow/create-transaction</p></> : <p>No quote found.</p>}{escrow && <><p>Escrow transaction: {escrow.escrow_transaction_id}</p><p>Status: {escrow.escrow_status}</p><p>Payment link: {escrow.payment_link ?? "N/A"}</p></>}</div>;
}
