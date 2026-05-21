import { redirect } from "next/navigation";
import { getDbPool } from "@/lib/db";
import { isOwnerAuthenticated } from "@/lib/owner-auth";

export const dynamic = "force-dynamic";

export default async function OwnerCommandCenterPage({ searchParams }: { searchParams: Promise<{ password?: string; rfq?: string }> }) {
  const params = await searchParams;
  if (!(await isOwnerAuthenticated(params.password ?? null))) redirect('/owner/login');
  const pool = getDbPool();
  const rfqs = pool ? (await pool.query("select id,full_name,company_name,city,product_interest,required_capacity_tph,status from quote_requests order by created_at desc limit 100")).rows : [];
  const selectedId = params.rfq || rfqs[0]?.id;
  const analyses = pool && selectedId ? (await pool.query("select * from ai_rfq_analyses where quote_request_id=$1 order by created_at desc limit 5", [selectedId])).rows : [];
  const market = pool && selectedId ? (await pool.query("select mrr.report_json,mrs.source_url from market_research_reports mrr join market_research_jobs mrj on mrj.id=mrr.job_id left join market_research_sources mrs on mrs.job_id=mrj.id where mrj.quote_request_id=$1 order by mrr.created_at desc", [selectedId])).rows : [];
  const messages = pool && selectedId ? (await pool.query("select * from supplier_messages where quote_request_id=$1 order by created_at desc limit 1", [selectedId])).rows : [];
  const escrow = pool && selectedId ? (await pool.query("select escrow_transaction_id,escrow_status,payment_link from escrow_transactions where quote_request_id=$1 order by created_at desc limit 1", [selectedId])).rows[0] : null;
  const milestones = pool && selectedId ? (await pool.query("select milestone_name,status from operation_milestones where quote_request_id=$1 order by created_at desc", [selectedId])).rows : [];
  return <main className="page-stack"><h1>Owner Command Center</h1><section className="card"><h2>RFQ Inbox</h2>{rfqs.map((r:any)=><article key={r.id}><a href={`?rfq=${r.id}`}>{r.full_name} / {r.company_name} / {r.city} / {r.product_interest} / {r.required_capacity_tph} / {r.status}</a><form action="/api/owner/ai/analyze-rfq" method="post"><input type="hidden" name="quote_request_id" value={r.id}/><input type="hidden" name="password" value={params.password??''}/><button className="btn">Analyze with AI</button></form></article>)}</section>
  <section className="card"><h2>AI Analysis</h2>{analyses.length?analyses.map((a:any)=><pre key={a.id}>{JSON.stringify(a.analysis_json,null,2)}</pre>):<p>Henüz AI analizi yok</p>}</section>
  <section className="card"><h2>Market Research</h2><form action="/api/owner/ai/market-research" method="post"><input type="hidden" name="quote_request_id" value={selectedId}/><input type="hidden" name="password" value={params.password??''}/><button className="btn">Run Market Research</button></form>{market.map((m:any,i:number)=><p key={i}><a href={m.source_url} target="_blank" rel="noreferrer">{m.source_url}</a></p>)}</section>
  <section className="card"><h2>Supplier Message</h2><form action="/api/owner/ai/generate-supplier-message" method="post"><input type="hidden" name="quote_request_id" value={selectedId}/><input type="hidden" name="password" value={params.password??''}/><button className="btn">Generate Supplier Message</button></form><pre>{messages[0]?.message_json ? JSON.stringify(messages[0].message_json,null,2):"Henüz kayıt yok"}</pre></section>
  <section className="card"><h2>Quote Builder</h2><p>/api/owner/quotes/calculate endpoint is active for owner-only use.</p></section>
  <section className="card"><h2>Escrow Preparation</h2><form action="/api/owner/escrow/prepare" method="post"><input type="hidden" name="quote_request_id" value={selectedId}/><input type="hidden" name="password" value={params.password??''}/><button className="btn">Prepare Escrow</button></form><p>{escrow?`${escrow.escrow_transaction_id} / ${escrow.escrow_status} / ${escrow.payment_link ?? '-'}`:'Henüz kayıt yok'}</p></section>
  <section className="card"><h2>Milestones</h2>{milestones.length?milestones.map((m:any,i:number)=><p key={i}>{m.milestone_name} - {m.status}</p>):<p>Henüz kayıt yok</p>}</section></main>;
}
