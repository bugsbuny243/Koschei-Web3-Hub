import { NextResponse } from "next/server";
import { getDbPool } from "@/lib/db";
import { analyzeQuoteRequestWithAi, buildSupplierMessageWithAi } from "@/lib/ai/tradepi-ai";

export async function POST(req: Request, { params }: { params: Promise<{ id: string }> }) {
  const { id } = await params;
  const url = new URL(req.url);

  if (url.searchParams.get("password") !== process.env.ADMIN_PASSWORD) {
    return NextResponse.json({ error: "unauthorized" }, { status: 401 });
  }

  const pool = getDbPool();
  if (!pool) {
    return NextResponse.json({ error: "db_unavailable" }, { status: 500 });
  }

  const row = (await pool.query("SELECT * FROM quote_requests WHERE id=$1", [id])).rows[0];
  if (!row) {
    return NextResponse.json({ error: "not_found" }, { status: 404 });
  }

  const analysis = await analyzeQuoteRequestWithAi({ quoteRequest: row });
  const supplierMessage = await buildSupplierMessageWithAi({ quoteRequest: row, analysis });

  return NextResponse.json({
    quote_request_id: id,
    model: process.env.TOGETHER_MODEL || "Qwen/Qwen3-Coder-480B-A35B-Instruct-FP8",
    analysis,
    supplier_message: supplierMessage,
  });
}
