import { redirect } from "next/navigation";
import { getDbPool } from "@/lib/db";
import { isOwnerAuthenticated } from "@/lib/owner-auth";
import SupplierOutreachClient from "./supplier-outreach-client";

export const dynamic = "force-dynamic";

export default async function SupplierOutreachPage({ searchParams }: { searchParams: Promise<{ password?: string }> }) {
  const params = await searchParams;
  if (!(await isOwnerAuthenticated(params.password ?? null))) redirect("/owner/login");
  const pool = getDbPool();
  const leads = pool ? (await pool.query(`select l.*,m.id as message_id,m.subject,m.body from supplier_leads l left join lateral (select id,subject,body from supplier_outreach_messages som where som.supplier_lead_id=l.id order by som.created_at desc limit 1) m on true order by l.created_at desc limit 200`)).rows : [];
  return <SupplierOutreachClient initialLeads={leads} braveConfigured={!!process.env.BRAVE_SEARCH_API_KEY} togetherConfigured={!!process.env.TOGETHER_API_KEY} dbConfigured={!!pool} fallbackPassword={params.password ?? ""} />;
}
