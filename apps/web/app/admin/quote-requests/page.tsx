import Link from "next/link";
import { getDbPool } from "@/lib/db";

export default async function AdminQuoteRequests({ searchParams }: { searchParams: Promise<{ password?: string }> }) {
  const { password } = await searchParams;
  if (!process.env.ADMIN_PASSWORD || password !== process.env.ADMIN_PASSWORD) return <div className="page-stack"><h1>Admin access required</h1></div>;
  const pool = getDbPool();
  const rows = pool ? (await pool.query("SELECT * FROM quote_requests ORDER BY created_at DESC LIMIT 100")).rows : [];
  return <div className="page-stack"><h1>Admin RFQ Workflow</h1>{rows.map((r:any)=><article key={r.id} className="card"><h3>{r.company_name} / {r.full_name}</h3><p>{r.country}, {r.city}, {r.district} - {r.full_delivery_address}</p><p>Business: {r.business_type} | Crops: {r.crop_types} | Capacity: {r.required_capacity_tph}</p><p>Config: cabinet {String(r.need_control_cabinet)}, fan/cyclone {String(r.need_fan_cyclone)}, elevator {String(r.need_bucket_elevator)}, spare screens {String(r.need_spare_screen_sets)}</p><p>Logistics: {r.preferred_trade_term}, {r.destination_port}, unloading {String(r.forklift_or_unloading_available)}, customs support {String(r.customs_support_needed)}</p><p>Status: {r.status}</p><form action={`/api/admin/quote-requests/${r.id}/supplier-request?password=${password}`} method="post"><button className="btn">Create supplier request</button></form><p><Link href={`/admin/quote-requests/${r.id}/pricing?password=${password}`}>Pricing workflow</Link> | <Link href={`/admin/quote-requests/${r.id}/escrow?password=${password}`}>Escrow</Link> | <Link href={`/admin/quote-requests/${r.id}/supplier-payments?password=${password}`}>Supplier payments</Link></p></article>)}</div>;
}
