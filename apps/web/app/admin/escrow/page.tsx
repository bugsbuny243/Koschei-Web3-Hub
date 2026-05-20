export default async function AdminEscrowPage({ searchParams }: { searchParams: Promise<{ password?: string }> }) {
  const { password } = await searchParams;
  if (!process.env.ADMIN_PASSWORD || password !== process.env.ADMIN_PASSWORD) {
    return <div className="page-stack"><h1>Admin Access Required</h1></div>;
  }

  const rows = [
    ["ESCROW_ENV", process.env.ESCROW_ENV || "not set"],
    ["ESCROW_API_BASE_URL exists", process.env.ESCROW_API_BASE_URL ? "yes" : "no"],
    ["ESCROW_EMAIL exists", process.env.ESCROW_EMAIL ? "yes" : "no"],
    ["ESCROW_API_KEY exists", process.env.ESCROW_API_KEY ? "yes" : "no"],
    ["ESCROW_DEFAULT_SELLER_EMAIL exists", process.env.ESCROW_DEFAULT_SELLER_EMAIL ? "yes" : "no"],
    ["ESCROW_DEFAULT_CURRENCY", process.env.ESCROW_DEFAULT_CURRENCY || "not set"],
    ["ESCROW_FEE_PAYER", process.env.ESCROW_FEE_PAYER || "not set"]
  ] as const;

  return (
    <div className="page-stack">
      <h1>Escrow Admin Diagnostics</h1>
      <p>Escrow.com fees are paid by TradePi Globall as internal operating cost.</p>
      <div className="card">
        {rows.map(([k, v]) => (
          <p key={k}><strong>{k}:</strong> {v}</p>
        ))}
      </div>
    </div>
  );
}
