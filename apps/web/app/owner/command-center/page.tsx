import { redirect } from "next/navigation";
import { getDbPool } from "@/lib/db";
import { isOwnerAuthenticated } from "@/lib/owner-auth";
import { CommandCenterClient } from "./command-center-client";

export const dynamic = "force-dynamic";

export default async function OwnerCommandCenterPage({ searchParams }: { searchParams: Promise<{ password?: string; rfq?: string }> }) {
  const params = await searchParams;
  if (!(await isOwnerAuthenticated(params.password ?? null))) redirect("/owner/login");
  const pool = getDbPool();
  const rfqs = pool
    ? (
        await pool.query(
          "select id,full_name,company_name,email,city,delivery_address,notes,crop,product_interest,required_capacity_tph,status,created_at from quote_requests order by created_at desc limit 100",
        )
      ).rows
    : [];
  const selectedId = params.rfq || rfqs[0]?.id || null;

  const analyses = pool && selectedId ? (await pool.query("select id,analysis_json,created_at from ai_rfq_analyses where quote_request_id=$1 order by created_at desc limit 5", [selectedId])).rows : [];
  const market =
    pool && selectedId
      ? (
          await pool.query(
            "select mrr.report_json,mrs.source_url,mrr.created_at from market_research_reports mrr join market_research_jobs mrj on mrj.id=mrr.job_id left join market_research_sources mrs on mrs.job_id=mrj.id where mrj.quote_request_id=$1 order by mrr.created_at desc",
            [selectedId],
          )
        ).rows
      : [];
  const messages = pool && selectedId ? (await pool.query("select id,message_json,created_at from supplier_messages where quote_request_id=$1 order by created_at desc limit 1", [selectedId])).rows : [];
  const escrow =
    pool && selectedId
      ? (
          await pool.query(
            "select escrow_transaction_id,escrow_status,payment_link from escrow_transactions where quote_request_id=$1 order by created_at desc limit 1",
            [selectedId],
          )
        ).rows[0]
      : null;
  const milestones =
    pool && selectedId ? (await pool.query("select milestone_name,status,created_at from operation_milestones where quote_request_id=$1 order by created_at asc", [selectedId])).rows : [];

  return (
    <CommandCenterClient
      fallbackPassword={params.password ?? ""}
      initialRfqs={rfqs}
      initialSelectedId={selectedId}
      initialAnalyses={analyses}
      initialMarket={market}
      initialMessages={messages}
      initialEscrow={escrow}
      initialMilestones={milestones}
    />
  );
}
