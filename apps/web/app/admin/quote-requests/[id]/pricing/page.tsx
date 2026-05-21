export default async function PricingPage({ params, searchParams }: { params: Promise<{ id: string }>; searchParams: Promise<{ password?: string }> }) {
  const { id } = await params;
  const { password } = await searchParams;

  if (!process.env.ADMIN_PASSWORD || password !== process.env.ADMIN_PASSWORD) {
    return <div className="page-stack"><h1>Admin Access Required</h1></div>;
  }

  return (
    <div className="page-stack">
      <h1>TradePi Pricing Workflow</h1>

      <form className="card" action={`/api/admin/quote-requests/${id}/supplier-ddp-quote?password=${password}`} method="post">
        <h2>1) Supplier Raw DDP Cost (Private)</h2>
        <input name="supplier_name" placeholder="supplier_name" className="input" />
        <input name="currency" defaultValue="USD" placeholder="currency" className="input" />
        <input name="raw_ddp_cost" type="number" min="0" step="0.01" required placeholder="raw_ddp_cost" className="input" />
        <input name="destination" placeholder="destination" className="input" />
        <textarea name="supplier_notes" placeholder="supplier_notes" className="input" />
        <button className="btn" type="submit">Save supplier DDP quote</button>
      </form>

      <form className="card" action={`/api/admin/quote-requests/${id}/customer-final-quote?password=${password}`} method="post">
        <h2>2) Customer Final Quote</h2>
        <input name="tradepi_margin" type="number" min="0" step="0.01" required placeholder="tradepi_margin" className="input" />
        <input name="escrow_fee_buffer" type="number" min="0" step="0.01" required placeholder="escrow_fee_buffer" className="input" />
        <input name="bank_fx_risk_buffer" type="number" min="0" step="0.01" required placeholder="bank_fx_risk_buffer" className="input" />
        <input name="payment_terms_public" placeholder="payment_terms_public" className="input" />
        <input name="delivery_terms_public" placeholder="delivery_terms_public" className="input" />
        <textarea name="quote_notes" placeholder="quote_notes" className="input" />
        <button className="btn btn-primary" type="submit">Create customer final quote</button>
      </form>
    </div>
  );
}
