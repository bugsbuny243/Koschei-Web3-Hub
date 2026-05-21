import { getDbPool } from "@/lib/db";
import { isOwnerRequest } from "@/lib/owner-command-center";
import SupplierOutreachClient from "./supplier-outreach-client";

export const dynamic = "force-dynamic";

export default async function SupplierOutreachPage({ searchParams }: { searchParams: Promise<{ password?: string }> }) {
  const params = await searchParams;
  if (!isOwnerRequest(params.password ?? null)) return <main className="page-stack"><h1>Supplier Outreach</h1><p>Unauthorized.</p></main>;

  const pool = getDbPool();
  const leads = pool ? (await pool.query("select l.*, a.product_fit,a.risk_notes,a.recommended_action,m.id as message_id,m.subject,m.body,m.approved_by_owner from supplier_leads l left join lateral (select * from supplier_ai_analyses a where a.supplier_lead_id=l.id order by created_at desc limit 1) a on true left join lateral (select * from supplier_outreach_messages m where m.supplier_lead_id=l.id order by created_at desc limit 1) m on true order by l.created_at desc limit 200")).rows : [];

  return <SupplierOutreachClient password={params.password ?? ""} initialLeads={leads} braveConfigured={!!process.env.BRAVE_SEARCH_API_KEY} togetherConfigured={!!process.env.TOGETHER_API_KEY} dbConfigured={!!pool} />;
}
