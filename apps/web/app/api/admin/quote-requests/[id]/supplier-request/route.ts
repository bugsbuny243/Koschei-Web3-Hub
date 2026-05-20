import { NextResponse } from "next/server";
import { getDbPool } from "@/lib/db";

export async function POST(req: Request, { params }: { params: Promise<{ id: string }> }) {
  const { id } = await params;
  const url = new URL(req.url);
  if (url.searchParams.get("password") !== process.env.ADMIN_PASSWORD) return NextResponse.json({ error: "unauthorized" }, { status: 401 });
  const pool = getDbPool();
  if (!pool) return NextResponse.redirect(new URL(`/admin/quote-requests?password=${url.searchParams.get("password")}`, req.url));
  const r = (await pool.query("SELECT * FROM quote_requests WHERE id=$1", [id])).rows[0];
  const summary = `Supplier Cost Request - Fine Cleaner 5X-5\nDestination: ${r.country}/${r.city}/${r.district}\nFull delivery destination: ${r.full_delivery_address || r.delivery_address_details}\nCrop types: ${r.crop_types}\nRequired capacity: ${r.required_capacity_tph}\nRequired screen sets: ${r.requested_screen_sets || "N/A"}\nRequested configuration: control cabinet ${r.need_control_cabinet}, fan/cyclone ${r.need_fan_cyclone}, bucket elevator ${r.need_bucket_elevator}, spare screens ${r.need_spare_screen_sets}\nVoltage: ${r.voltage_available}\nInstallation location: ${r.installation_location_type}\nPreferred trade term: ${r.preferred_trade_term}\nUnloading/customs notes: forklift ${r.forklift_or_unloading_available}, customs support ${r.customs_support_needed}\nSpecial requirements: ${r.special_requirements || "N/A"}`;
  await pool.query("INSERT INTO supplier_cost_requests (quote_request_id,supplier_name,supplier_contact_name,product_slug,request_summary,status) VALUES ($1,$2,$3,$4,$5,'ready_to_send')", [id, "Chinese Supplier", "Cathy", "fine-cleaner-5x-5", summary]);
  await pool.query("UPDATE quote_requests SET status='supplier_cost_requested' WHERE id=$1", [id]);
  return NextResponse.redirect(new URL(`/admin/quote-requests?password=${url.searchParams.get("password")}`, req.url));
}
