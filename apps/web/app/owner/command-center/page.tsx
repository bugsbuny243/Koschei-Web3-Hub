import { getDbPool } from "@/lib/db";
import { isOwnerRequest } from "@/lib/owner-command-center";

export const dynamic = "force-dynamic";

export default async function OwnerCommandCenterPage({ searchParams }: { searchParams: Promise<{ password?: string }> }) {
  const params = await searchParams;
  if (!isOwnerRequest(params.password ?? null)) {
    return <main className="page-stack"><h1>Owner Command Center</h1><p>Unauthorized.</p></main>;
  }

  const pool = getDbPool();
  const rfqs = pool ? (await pool.query("select id,full_name,company_name,city,crop_types,required_capacity_tph,product_interest,status,created_at from quote_requests order by created_at desc limit 100")).rows : [];

  return (
    <main className="page-stack">
      <h1>TradePi AI Command Center (Owner)</h1>
      <section className="card">
        <h2>1. RFQ Inbox</h2>
        {rfqs.map((r: any) => (
          <article key={r.id} className="card">
            <p><strong>{r.full_name}</strong> / {r.company_name} / {r.city}</p>
            <p>Crop: {r.crop_types} | Capacity: {r.required_capacity_tph} | Product: {r.product_interest}</p>
            <p>Status: <span className="badge">{r.status}</span></p>
            <form action="/api/owner/ai/analyze-rfq" method="post">
              <input type="hidden" name="password" value={params.password} />
              <input type="hidden" name="quote_request_id" value={r.id} />
              <button className="btn">Analyze with AI</button>
            </form>
          </article>
        ))}
      </section>
      <section className="card"><h2>2-7. AI Analysis / Market / Supplier Message / Quote Builder / Escrow / Timeline</h2><p>Use owner API routes for secure, server-side operations.</p></section>
    </main>
  );
}
