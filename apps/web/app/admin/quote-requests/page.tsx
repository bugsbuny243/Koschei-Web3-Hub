import { query } from "@/lib/db";

type QR = { created_at: string; full_name: string; company_name: string; country: string; city: string; product_interest: string; required_capacity_tph: string; preferred_trade_term: string; status: string; admin_notes: string };

export default async function AdminQuoteRequests({ searchParams }: { searchParams: Promise<Record<string,string|undefined>> }) {
  const sp = await searchParams;
  if (!process.env.ADMIN_PASSWORD || sp.password !== process.env.ADMIN_PASSWORD) {
    return <main className="mx-auto max-w-md px-4 py-10"><h1 className="text-2xl font-bold">Admin Access</h1><form className="mt-4"><input name="password" type="password" placeholder="ADMIN_PASSWORD" className="w-full rounded border px-3 py-2"/><button className="mt-3 rounded bg-slate-900 px-4 py-2 text-white">Open</button></form></main>;
  }
  const result = await query<QR>("select created_at,full_name,company_name,country,city,product_interest,required_capacity_tph,preferred_trade_term,status,admin_notes from quote_requests order by created_at desc limit 200");
  return <main className="mx-auto max-w-6xl px-4 py-8 sm:px-6"><h1 className="text-3xl font-bold">Quote Requests</h1><div className="mt-4 overflow-x-auto"><table className="min-w-full border bg-white text-sm"><thead><tr>{["date","customer","company","country/city","product","capacity","trade term","status","notes"].map((h)=><th key={h} className="border px-2 py-2 text-left">{h}</th>)}</tr></thead><tbody>{result.rows.map((r,i)=><tr key={i}><td className="border px-2 py-2">{new Date(r.created_at).toLocaleDateString()}</td><td className="border px-2 py-2">{r.full_name}</td><td className="border px-2 py-2">{r.company_name}</td><td className="border px-2 py-2">{r.country}/{r.city}</td><td className="border px-2 py-2">{r.product_interest}</td><td className="border px-2 py-2">{r.required_capacity_tph}</td><td className="border px-2 py-2">{r.preferred_trade_term}</td><td className="border px-2 py-2">{r.status}</td><td className="border px-2 py-2">{r.admin_notes}</td></tr>)}</tbody></table></div></main>;
}
