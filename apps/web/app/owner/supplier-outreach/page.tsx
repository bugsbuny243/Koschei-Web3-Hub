import { getDbPool } from "@/lib/db";
import { isOwnerRequest } from "@/lib/owner-command-center";

export const dynamic = "force-dynamic";

const defaultCategories = [
  "Seed cleaning machines",
  "Grain cleaning machines",
  "Air screen cleaners",
  "Fine cleaners",
  "Gravity separators",
  "De-stoners",
  "Color sorters",
  "Bucket elevators",
  "Packing scales",
  "Grain silos",
  "Seed processing lines",
];

const leadStatuses = [
  "new",
  "reviewed",
  "message_ready",
  "sent",
  "replied",
  "interested",
  "rejected",
  "not_relevant",
  "blocked",
];

type SupplierLeadRow = {
  id: string;
  company_name: string | null;
  platform: string | null;
  source_url: string;
  country: string | null;
  city: string | null;
  confidence: string | null;
  status: string | null;
  created_at: string;
};

export default async function SupplierOutreachPage({ searchParams }: { searchParams: Promise<{ password?: string }> }) {
  const params = await searchParams;
  if (!isOwnerRequest(params.password ?? null)) {
    return (
      <main className="page-stack">
        <h1>Supplier Outreach</h1>
        <p>Unauthorized.</p>
      </main>
    );
  }

  const pool = getDbPool();
  const leads: SupplierLeadRow[] = pool
    ? (
        (await pool.query(
          `select id, company_name, platform, source_url, country, city, confidence, status, created_at
           from supplier_leads
           order by created_at desc
           limit 200`,
        )
      ).rows as SupplierLeadRow[])
    : [];

  return (
    <main className="page-stack">
      <h1>TradePi Supplier Outreach Foundation (Owner-Only)</h1>
      <p>Phase 1 foundation only. No auto-send. Owner-reviewed copy-ready workflow.</p>

      <section className="card">
        <h2>1. Supplier Discovery Form</h2>
        <form>
          <label>
            Product Category
            <select defaultValue={defaultCategories[0]}>
              {defaultCategories.map((category) => (
                <option key={category} value={category}>
                  {category}
                </option>
              ))}
            </select>
          </label>
          <label>
            Keywords
            <input name="keywords" placeholder="seed cleaning machine manufacturer China" />
          </label>
          <label>
            Target Country
            <input name="target_country" defaultValue="China" />
          </label>
          <label>
            Target Platform
            <input name="target_platform" placeholder="Alibaba / Made-in-China / Global Sources (manual)" />
          </label>
          <label>
            Minimum Confidence
            <select defaultValue="low">
              <option value="low">low</option>
              <option value="medium">medium</option>
              <option value="high">high</option>
            </select>
          </label>
          <label>
            Notes
            <textarea name="notes" rows={3} placeholder="Owner notes for discovery run." />
          </label>
          <p>Placeholder only. Discovery execution and search integrations are intentionally disabled in Phase 1.</p>
          <button className="btn" type="button" disabled>
            Run Discovery (Coming Soon)
          </button>
        </form>
      </section>

      <section className="card">
        <h2>2. Supplier Leads Table</h2>
        <div style={{ overflowX: "auto" }}>
          <table>
            <thead>
              <tr>
                <th>Company</th>
                <th>Platform</th>
                <th>Country/City</th>
                <th>Confidence</th>
                <th>Status</th>
                <th>Source URL</th>
              </tr>
            </thead>
            <tbody>
              {leads.length === 0 ? (
                <tr>
                  <td colSpan={6}>No supplier leads yet.</td>
                </tr>
              ) : (
                leads.map((lead) => (
                  <tr key={lead.id}>
                    <td>{lead.company_name ?? "Unknown"}</td>
                    <td>{lead.platform ?? "Unknown"}</td>
                    <td>{[lead.country, lead.city].filter(Boolean).join(" / ") || "Unknown"}</td>
                    <td>{lead.confidence ?? "low"}</td>
                    <td>{lead.status ?? "new"}</td>
                    <td>
                      <a href={lead.source_url} target="_blank" rel="noreferrer">
                        {lead.source_url}
                      </a>
                    </td>
                  </tr>
                ))
              )}
            </tbody>
          </table>
        </div>
      </section>

      <section className="card">
        <h2>3. Selected Lead Detail</h2>
        <p>Placeholder: selecting a lead and showing full profile details will be added in next phase.</p>
      </section>

      <section className="card">
        <h2>4. AI Analysis placeholder</h2>
        <p>Placeholder only. Together/Qwen integration is intentionally not implemented in Phase 1.</p>
      </section>

      <section className="card">
        <h2>5. Outreach Message placeholder</h2>
        <p>Placeholder only. Draft generation and message sending are intentionally disabled in Phase 1.</p>
      </section>

  return <SupplierOutreachClient password={params.password ?? ""} initialLeads={leads} braveConfigured={!!process.env.BRAVE_SEARCH_API_KEY} togetherConfigured={!!process.env.TOGETHER_API_KEY} dbConfigured={!!pool} />;
}
