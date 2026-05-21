"use client";
import { useState } from "react";

const defaults = ["Seed cleaning machines", "Grain cleaning machines", "Air screen cleaners"];
const statuses = ["new", "reviewed", "message_ready", "sent", "replied", "interested", "rejected", "not_relevant", "blocked"];

export default function SupplierOutreachClient({ password, initialLeads, braveConfigured, togetherConfigured, dbConfigured }: { password: string; initialLeads: any[]; braveConfigured: boolean; togetherConfigured: boolean; dbConfigured: boolean }) {
  const [leads, setLeads] = useState<any[]>(initialLeads);
  const [keywords, setKeywords] = useState("seed cleaning machine manufacturer China");
  const [productCategory, setProductCategory] = useState(defaults[0]);
  const [error, setError] = useState("");

  async function post(path: string, payload: any) {
    const res = await fetch(path, { method: "POST", headers: { "content-type": "application/json" }, body: JSON.stringify({ ...payload, password }) });
    const out = await res.json();
    if (!res.ok || out?.error) throw new Error(out?.error || "Request failed");
    return out;
  }

  return <main className="page-stack"><h1>TradePi Supplier Outreach Agent (Owner)</h1>
    {!braveConfigured && <p>Brave Search API is not configured.</p>}
    {!togetherConfigured && <p>Together API is not configured.</p>}
    {!dbConfigured && <p>Database is not configured.</p>}
    {error && <p>{error}</p>}
    <section className="card"><h2>Discovery Form</h2>
      <select value={productCategory} onChange={(e) => setProductCategory(e.target.value)}>{defaults.map((c) => <option key={c}>{c}</option>)}</select>
      <input value={keywords} onChange={(e) => setKeywords(e.target.value)} />
      <button className="btn" onClick={async () => { setError(""); try { const out = await post("/api/owner/supplier-outreach/discover", { product_category: productCategory, keywords, target_country: "China", target_platform: "mixed" }); setLeads((p) => [...out.leads, ...p]); if (!out.leads?.length) setError("No leads found"); } catch (e: any) { setError(e.message); } }}>Discover Suppliers</button>
    </section>
    <section className="card"><h2>Search Results / Leads</h2>{leads.map((l) => <article className="card" key={l.id}><p><strong>{l.company_name || l.possible_company_name || "Unknown"}</strong> <span className="badge">{l.status}</span></p><p><a href={l.source_url} target="_blank">{l.source_url}</a></p><p>Platform: {l.platform} | Confidence: {l.confidence}</p>
      <h3>AI Analysis</h3><p>Manufacturer score: {String(l.manufacturer_score ?? "-")} | Risk score: {String(l.risk_score ?? "-")}</p><p>Likely manufacturer: {String(l.likely_manufacturer)} | Likely trader: {String(l.likely_trader)}</p><p>Product fit: {l.product_fit ?? "unknown"}</p><p>Risk notes: {l.risk_notes ?? ""}</p><p>Recommended action: {l.recommended_action ?? ""}</p>
      <h3>Outreach Draft</h3>{l.body ? <textarea readOnly rows={8} value={`Subject: ${l.subject}\n\n${l.body}`} /> : <p>No draft yet.</p>}
      <h3>Manual Status Tracking</h3><div style={{ display: "flex", gap: 8, flexWrap: "wrap" }}>
      <button className="btn" onClick={async () => { setError(""); try { await post("/api/owner/supplier-outreach/analyze", { supplier_lead_id: l.id }); location.reload(); } catch (e: any) { setError(e.message); } }}>Analyze with AI</button>
      <button className="btn" onClick={async () => { setError(""); try { await post("/api/owner/supplier-outreach/generate-message", { supplier_lead_id: l.id }); location.reload(); } catch (e: any) { setError(e.message); } }}>Generate Message</button>
      <button className="btn" onClick={async () => { setError(""); try { if (l.message_id) await post("/api/owner/supplier-outreach/approve-message", { message_id: l.message_id }); location.reload(); } catch (e: any) { setError(e.message); } }}>Approve Message</button>
      <button className="btn" onClick={async () => { await navigator.clipboard.writeText(`Subject: ${l.subject || ""}\n\n${l.body || ""}`); await post("/api/owner/supplier-outreach/event", { supplier_lead_id: l.id, event_type: "copied_message" }); }}>Copy Message</button>
      <select defaultValue={l.status} onChange={async (e) => { setError(""); try { await post("/api/owner/supplier-outreach/mark-status", { supplier_lead_id: l.id, status: e.target.value }); location.reload(); } catch (err: any) { setError(err.message); } }}>{statuses.map((s) => <option key={s}>{s}</option>)}</select>
      </div></article>)}</section>
    <section className="card"><p>Makine görseli hazırlanıyor</p></section>
  </main>;
}
