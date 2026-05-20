import { NextResponse } from "next/server";
import { query } from "@/lib/db";
import crypto from "node:crypto";

export async function POST(req: Request) {
  const form = await req.formData();
  if (!form.get("consent")) return NextResponse.json({ error: "Consent required" }, { status: 400 });
  const id = crypto.randomUUID();
  await query(`insert into quote_requests (id,full_name,company_name,email,phone,country,city,product_interest,raw_material_type,required_capacity_tph,target_quantity,preferred_trade_term,destination_port_or_city,message,metadata) values ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15)`, [
    id, form.get("full_name"), form.get("company_name"), form.get("email"), form.get("phone"), form.get("country"), form.get("city"), form.get("product_interest"), form.get("raw_material_type"), form.get("required_capacity_tph"), form.get("target_quantity"), form.get("preferred_trade_term"), form.get("destination_port_or_city"), form.get("message"), JSON.stringify({ file_upload_url: form.get("file_upload_url") || null })
  ]);
  return NextResponse.redirect(new URL("/request-quote/success", req.url));
}
