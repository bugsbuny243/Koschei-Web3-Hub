import { NextResponse } from "next/server";
import { getDbPool } from "@/lib/db";
import { isOwnerRequest } from "@/lib/owner-command-center";
import { normalizeSupplierUrl, searchSuppliers } from "@/lib/research/brave-search";

export async function POST(req: Request) {
  const body = await req.json().catch(() => ({}));
  if (!isOwnerRequest(body.password ?? null)) return NextResponse.json({ error: "unauthorized" }, { status: 401 });
  if (!process.env.BRAVE_SEARCH_API_KEY) return NextResponse.json({ error: "Brave Search API is not configured." }, { status: 400 });
  const pool = getDbPool();
  if (!pool) return NextResponse.json({ error: "Database is not configured." }, { status: 500 });

  const keywords = String(body.keywords || "").trim();
  const productCategory = String(body.product_category || "").trim();
  if (!keywords) return NextResponse.json({ error: "keywords required" }, { status: 400 });

  const queries = [...new Set([
    keywords,
    productCategory ? `${productCategory} manufacturer China` : "",
    productCategory ? `${productCategory} Made-in-China manufacturer` : "",
    productCategory ? `${productCategory} Alibaba manufacturer` : "",
    productCategory ? `${productCategory} factory China` : "",
  ].filter(Boolean))].slice(0, 5);

  const job = (await pool.query(
    "insert into supplier_discovery_jobs (product_category,keywords,target_country,target_platform,search_query,created_by,status,started_at) values ($1,$2,$3,$4,$5,$6,'running',now()) returning *",
    [body.product_category ?? null, body.keywords ?? null, body.target_country ?? "China", body.target_platform ?? null, queries.join(" | "), "owner"],
  )).rows[0];

  try {
    const discovered = [];
    for (const query of queries) discovered.push(...(await searchSuppliers(query)));

    const leads: any[] = [];
    for (const r of discovered) {
      const normalizedUrl = normalizeSupplierUrl(r.url);
      const existing = (await pool.query("select * from supplier_leads where source_url=$1 order by created_at asc limit 1", [normalizedUrl])).rows[0];
      const lead = existing || (await pool.query(
        "insert into supplier_leads (discovery_job_id,company_name,possible_company_name,platform,source_url,country,product_categories,confidence,notes) values ($1,$2,$3,$4,$5,$6,$7,'low',$8) returning *",
        [job.id, null, r.title || null, r.platform, normalizedUrl, body.target_country ?? "China", body.product_category ? [body.product_category] : [], body.notes ?? null],
      )).rows[0];

      const sourceExists = (await pool.query("select 1 from supplier_lead_sources where supplier_lead_id=$1 and source_url=$2 and search_query=$3 limit 1", [lead.id, normalizedUrl, r.search_query])).rows[0];
      if (!sourceExists) {
        await pool.query("insert into supplier_lead_sources (supplier_lead_id,source_title,source_url,source_snippet,search_query,platform) values ($1,$2,$3,$4,$5,$6)", [lead.id, r.title, normalizedUrl, r.snippet, r.search_query, r.platform]);
      }

      if (!existing) leads.push(lead);
      await pool.query("insert into supplier_outreach_events (supplier_lead_id,event_type,note,created_by) values ($1,'discovered',$2,'owner')", [lead.id, `query=${r.search_query}`]);
    }

    await pool.query("update supplier_discovery_jobs set status='completed',finished_at=now(),updated_at=now() where id=$1", [job.id]);
    if (leads.length === 0) return NextResponse.json({ ok: true, job, leads: [], error: "No leads found" });
    return NextResponse.json({ ok: true, job, leads });
  } catch (e: any) {
    await pool.query("update supplier_discovery_jobs set status='failed',error_message=$2,finished_at=now(),updated_at=now() where id=$1", [job.id, String(e?.message || e)]);
    return NextResponse.json({ error: String(e?.message || e) }, { status: 500 });
  }
}
