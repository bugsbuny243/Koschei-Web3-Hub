import { NextResponse } from "next/server";
import { togetherChatJson } from "@/lib/ai/together-client";
import { getDbPool } from "@/lib/db";
import { isOwnerRequest } from "@/lib/owner-command-center";

type BraveResult = { title?: string; url?: string; description?: string };

export async function POST(req: Request) {
  const body = await req.json().catch(() => ({}));
  if (!isOwnerRequest(body.password ?? null)) return NextResponse.json({ error: "unauthorized" }, { status: 401 });
  const q = String(body.query || "Turkish agricultural machinery manufacturers");
  const qrId = String(body.quote_request_id || "");
  const apiKey = process.env.BRAVE_SEARCH_API_KEY;
  if (!apiKey) return NextResponse.json({ error: "BRAVE_SEARCH_API_KEY missing" }, { status: 500 });

  const res = await fetch(`https://api.search.brave.com/res/v1/web/search?q=${encodeURIComponent(q)}`, {
    headers: { "X-Subscription-Token": apiKey, Accept: "application/json" }, cache: "no-store",
  });
  if (!res.ok) return NextResponse.json({ error: `brave failed ${res.status}` }, { status: 502 });
  const data = await res.json();
  const sources: BraveResult[] = (data?.web?.results || []).slice(0, 8).map((x: any) => ({ title: x.title, url: x.url, description: x.description }));

  const report = await togetherChatJson<Record<string, unknown>>([
    { role: "system", content: "Analyze only provided sources. Never invent manufacturers/prices. Output JSON only." },
    { role: "user", content: `Create Turkish market research report from these Brave sources only:\n${JSON.stringify(sources)}` },
  ]);

  const pool = getDbPool();
  if (pool) {
    const job = (await pool.query("insert into market_research_jobs (quote_request_id,query,status) values ($1,$2,'completed') returning id", [qrId || null, q])).rows[0];
    for (const s of sources) {
      await pool.query("insert into market_research_sources (job_id,source_url,title,snippet) values ($1,$2,$3,$4)", [job.id, s.url ?? null, s.title ?? null, s.description ?? null]);
    }
    await pool.query("insert into market_research_reports (job_id,report_json) values ($1,$2)", [job.id, JSON.stringify(report)]);
  }

  return NextResponse.json({ ok: true, query: q, sources, report });
}
