import { NextResponse } from "next/server";
import { getDbPool } from "@/lib/db";
import { isOwnerRequest } from "@/lib/owner-command-center";
import { searchSuppliers } from "@/lib/research/brave-search";

export async function POST(req: Request) {
  const body = await req.json().catch(() => ({}));
  if (!isOwnerRequest(body.password ?? null)) return NextResponse.json({ error: "unauthorized" }, { status: 401 });
  if (!process.env.BRAVE_SEARCH_API_KEY) return NextResponse.json({ error: "Brave Search API is not configured." }, { status: 400 });
  const pool = getDbPool();
  if (!pool) return NextResponse.json({ error: "db not configured" }, { status: 500 });

  const query = String(body.keywords || "").trim();
  if (!query) return NextResponse.json({ error: "keywords required" }, { status: 400 });

  const job = (await pool.query(
    "insert into supplier_discovery_jobs (product_category,keywords,target_country,target_platform,search_query,created_by,status,started_at) values ($1,$2,$3,$4,$5,$6,'running',now()) returning *",
    [body.product_category ?? null, body.keywords ?? null, body.target_country ?? "China", body.target_platform ?? null, query, "owner"],
  )).rows[0];

  try {
    const results = await searchSuppliers(query);
    const leads: any[] = [];
    for (const r of results) {
      const lead = (await pool.query(
        "insert into supplier_leads (discovery_job_id,company_name,platform,source_url,country,product_categories,confidence,notes) values ($1,$2,$3,$4,$5,$6,'low',$7) returning *",
        [job.id, r.title || "Unknown", r.platform, r.url, body.target_country ?? "China", body.product_category ? [body.product_category] : [], body.notes ?? null],
      )).rows[0];
      await pool.query("insert into supplier_lead_sources (supplier_lead_id,source_title,source_url,source_snippet,search_query,platform) values ($1,$2,$3,$4,$5,$6)", [lead.id, r.title, r.url, r.snippet, r.search_query, r.platform]);
      leads.push(lead);
    }
    await pool.query("update supplier_discovery_jobs set status='completed',finished_at=now(),updated_at=now() where id=$1", [job.id]);
    return NextResponse.json({ ok: true, job, leads });
  } catch (e: any) {
    await pool.query("update supplier_discovery_jobs set status='failed',error_message=$2,finished_at=now(),updated_at=now() where id=$1", [job.id, String(e?.message || e)]);
    return NextResponse.json({ error: String(e?.message || e) }, { status: 500 });
  }
}
