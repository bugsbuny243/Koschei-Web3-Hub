"use client";
import { useState } from "react";

const defaults = ["Seed cleaning machines","Grain cleaning machines","Air screen cleaners","Fine cleaners","Gravity separators","De-stoners","Color sorters","Bucket elevators","Packing scales","Grain silos","Seed processing lines"];
const keywordsDefault = "seed cleaning machine manufacturer China";

export default function SupplierOutreachClient({ password, initialLeads, braveConfigured }: { password: string; initialLeads: any[]; braveConfigured: boolean }) {
  const [leads, setLeads] = useState<any[]>(initialLeads);
  const [keywords, setKeywords] = useState(keywordsDefault);
  const [productCategory, setProductCategory] = useState(defaults[0]);

  async function post(path: string, payload: any) {
    const res = await fetch(path, { method: "POST", headers: { "content-type": "application/json" }, body: JSON.stringify({ ...payload, password }) });
    return res.json();
  }

  return <main className="page-stack"><h1>TradePi Supplier Outreach Agent (Owner)</h1>
    {!braveConfigured && <p>Brave Search API is not configured.</p>}
    <section className="card"><h2>Supplier Discovery Form</h2>
      <select value={productCategory} onChange={(e) => setProductCategory(e.target.value)}>{defaults.map((c) => <option key={c}>{c}</option>)}</select>
      <input value={keywords} onChange={(e) => setKeywords(e.target.value)} />
      <button className="btn" onClick={async () => { const out = await post("/api/owner/supplier-outreach/discover", { product_category: productCategory, keywords, target_country: "China", target_platform: "mixed" }); if (out.leads) setLeads((p) => [...out.leads, ...p]); }}>Discover Suppliers</button>
    </section>
    <section className="card"><h2>Lead Cards</h2>{leads.map((l) => <article className="card" key={l.id}><p><strong>{l.company_name}</strong> ({l.platform})</p><p>{l.source_url}</p><p>Confidence: {l.confidence} | Manufacturer: {String(l.likely_manufacturer)} | Trader: {String(l.likely_trader)}</p><p>Product fit: {l.product_fit ?? "unknown"}</p><p>Risk notes: {l.risk_notes ?? ""}</p><p>Recommended: {l.recommended_action ?? ""}</p>{l.body && <textarea readOnly rows={10} value={`Subject: ${l.subject}\n\n${l.body}`} />}
      <div style={{ display: "flex", gap: 8, flexWrap: "wrap" }}>
      <button className="btn" onClick={async () => { await post("/api/owner/supplier-outreach/analyze", { supplier_lead_id: l.id }); location.reload(); }}>Analyze with AI</button>
      <button className="btn" onClick={async () => { await post("/api/owner/supplier-outreach/generate-message", { supplier_lead_id: l.id }); location.reload(); }}>Generate Message</button>
      <button className="btn" onClick={async () => { if (l.message_id) await post("/api/owner/supplier-outreach/approve-message", { message_id: l.message_id }); location.reload(); }}>Approve Message</button>
      <button className="btn" onClick={async () => navigator.clipboard.writeText(`Subject: ${l.subject || ""}\n\n${l.body || ""}`)}>Copy Message</button>
      <button className="btn" onClick={async () => { await post("/api/owner/supplier-outreach/mark-status", { supplier_lead_id: l.id, status: "sent" }); location.reload(); }}>Mark Sent</button>
      <button className="btn" onClick={async () => { await post("/api/owner/supplier-outreach/mark-status", { supplier_lead_id: l.id, status: "replied" }); location.reload(); }}>Mark Replied</button>
      <button className="btn" onClick={async () => { await post("/api/owner/supplier-outreach/mark-status", { supplier_lead_id: l.id, status: "rejected" }); location.reload(); }}>Reject Lead</button>
      </div>
      </article>)}</section>
  </main>;
}
