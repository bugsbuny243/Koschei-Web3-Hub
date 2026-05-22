"use client";

import Link from "next/link";
import { useMemo, useState } from "react";

type AnyObj = Record<string, any>;

const MILESTONES = ["RFQ received", "AI analyzed", "Supplier contacted", "Supplier quote received", "Customer quote sent", "Customer approved", "Escrow created", "Payment funded", "Production started", "Shipment started", "Customs", "Delivered"];

function trError(error: string) {
  if (error.includes("missing") || error.includes("BRAVE_SEARCH_API_KEY")) return "API yapılandırması eksik";
  if (error.includes("unauthorized")) return "Yetkisiz işlem";
  if (error.includes("rfq not found")) return "RFQ bulunamadı";
  return "İşlem sırasında bir hata oluştu";
}

export function CommandCenterClient({ fallbackPassword, initialRfqs, initialSelectedId, initialAnalyses, initialMarket, initialMessages, initialEscrow, initialMilestones }: AnyObj) {
  const [selectedId, setSelectedId] = useState<string | null>(initialSelectedId);
  const [analyses, setAnalyses] = useState(initialAnalyses || []);
  const [market, setMarket] = useState(initialMarket || []);
  const [message, setMessage] = useState(initialMessages?.[0]?.message_json ? JSON.stringify(initialMessages[0].message_json, null, 2) : "");
  const [escrow, setEscrow] = useState(initialEscrow);
  const [milestones, setMilestones] = useState(initialMilestones || []);
  const [status, setStatus] = useState("");
  const [loading, setLoading] = useState<string | null>(null);
  const selected = useMemo(() => initialRfqs.find((x: AnyObj) => x.id === selectedId), [initialRfqs, selectedId]);
  const [quoteForm, setQuoteForm] = useState({ supplier_machine_cost: "", supplier_ddp_total_cost: "", tradepi_margin_type: "percent", tradepi_margin_value: "", escrow_fee_estimate: "", production_days: "", shipping_days: "", customs_days: "", quote_valid_until: "" });
  const [quoteResult, setQuoteResult] = useState<AnyObj | null>(null);

  const counters = {
    total: initialRfqs.length,
    waitingSupplierQuote: initialRfqs.filter((r: AnyObj) => String(r.status || "").toLowerCase().includes("supplier")).length,
    waitingCustomerApproval: initialRfqs.filter((r: AnyObj) => String(r.status || "").toLowerCase().includes("approval")).length,
    escrowPending: initialRfqs.filter((r: AnyObj) => String(r.status || "").toLowerCase().includes("escrow")).length,
    inProduction: initialRfqs.filter((r: AnyObj) => String(r.status || "").toLowerCase().includes("production")).length,
    delivered: initialRfqs.filter((r: AnyObj) => String(r.status || "").toLowerCase().includes("delivered")).length,
  };

  async function callApi(path: string, body: AnyObj, setter?: (data: AnyObj) => void) {
    if (!selectedId) return setStatus("Önce bir RFQ seç");
    setLoading(path);
    setStatus("");
    const res = await fetch(path, { method: "POST", headers: { "content-type": "application/json" }, body: JSON.stringify({ ...body, quote_request_id: selectedId, password: fallbackPassword }) });
    const data = await res.json().catch(() => ({}));
    if (!res.ok) {
      setStatus(trError(String(data.error || res.statusText)));
      setLoading(null);
      return;
    }
    if (setter) setter(data);
    setStatus("İşlem tamamlandı");
    setLoading(null);
  }

  return <main className="owner-dashboard">
    <section className="owner-top">
      <div><h1>TradePi Owner Command Center</h1><p>RFQ, supplier quote, AI analysis, escrow and delivery operations</p></div>
      <div className="owner-quick-actions"><Link className="btn btn-secondary" href="#rfq-inbox">RFQ Inbox</Link><Link className="btn btn-secondary" href="/owner/supplier-outreach">Supplier Outreach</Link><Link className="btn btn-secondary" href="/owner/media-library">Media Library</Link><button className="btn btn-primary" type="button" disabled={!selectedId}>New Quote</button></div>
    </section>

    <section className="owner-kpis">{[["Total RFQs", counters.total], ["Waiting Supplier Quote", counters.waitingSupplierQuote], ["Waiting Customer Approval", counters.waitingCustomerApproval], ["Escrow Pending", counters.escrowPending], ["In Production", counters.inProduction], ["Delivered", counters.delivered]].map(([k, v]) => <article key={String(k)} className="owner-kpi-card"><p>{k}</p><strong>{String(v)}</strong></article>)}</section>

    <section className="owner-main-grid">
      <article className="card" id="rfq-inbox"><h2>RFQ Inbox</h2>{initialRfqs.length === 0 ? <p>Henüz RFQ talebi yok. Yeni müşteri talepleri burada görünecek.</p> : <div className="owner-rfq-list">{initialRfqs.map((rfq: AnyObj) => <div key={rfq.id} className="owner-rfq-row"><div><strong>{rfq.company_name || rfq.full_name}</strong><p>{rfq.city} • {rfq.product_interest}</p><p>Kapasite: {rfq.required_capacity_tph ?? "-"}</p></div><div><span className="badge">{rfq.status || "open"}</span><p>{new Date(rfq.created_at).toLocaleDateString("tr-TR")}</p><button className="btn btn-secondary" type="button" onClick={() => setSelectedId(rfq.id)}>Select</button></div></div>)}</div>}</article>

      <article className="card"><h2>Selected RFQ Workspace</h2>
      <section><h3>1. Customer Request</h3>{selected ? <div className="grid"><p><b>Company:</b> {selected.company_name || "-"}</p><p><b>Contact:</b> {selected.full_name || "-"} / {selected.email || "-"}</p><p><b>City:</b> {selected.city || "-"}</p><p><b>Crop:</b> {selected.crop || selected.product_interest || "-"}</p><p><b>Capacity:</b> {selected.required_capacity_tph || "-"}</p><p><b>Delivery Address:</b> {selected.delivery_address || "-"}</p><p><b>Notes:</b> {selected.notes || "-"}</p></div> : <p>Önce bir RFQ seç</p>}</section>
      <section><h3>2. AI Analysis</h3><button className="btn btn-primary" disabled={!selectedId || loading === "/api/owner/ai/analyze-rfq"} onClick={() => callApi("/api/owner/ai/analyze-rfq", {}, (d) => setAnalyses([{ analysis_json: d.analysis }, ...analyses]))}>{loading === "/api/owner/ai/analyze-rfq" ? "Yükleniyor..." : "Analyze with AI"}</button>{analyses.length ? <pre>{JSON.stringify(analyses[0].analysis_json, null, 2)}</pre> : <p>Henüz AI analizi yok</p>}</section>
      <section><h3>3. Market Research</h3><button className="btn btn-primary" disabled={!selectedId || loading === "/api/owner/ai/market-research"} onClick={() => callApi("/api/owner/ai/market-research", { query: `${selected?.product_interest || "machinery"} suppliers Turkey` }, (d) => setMarket((d.sources || []).map((s: AnyObj) => ({ source_url: s.url }))))}>Run Market Research</button>{market.length ? <ul>{market.map((m: AnyObj, i: number) => <li key={i}><a href={m.source_url} target="_blank" rel="noreferrer">{m.source_url}</a></li>)}</ul> : <p>Henüz piyasa araştırması yok</p>}</section>
      <section><h3>4. Supplier Message</h3><button className="btn btn-primary" disabled={!selectedId || loading === "/api/owner/ai/generate-supplier-message"} onClick={() => callApi("/api/owner/ai/generate-supplier-message", {}, (d) => setMessage(JSON.stringify(d.message, null, 2)))}>Generate Supplier Message</button><textarea value={message} onChange={(e) => setMessage(e.target.value)} rows={6} /> <button className="btn btn-secondary" type="button" onClick={async () => { await navigator.clipboard.writeText(message); setStatus("Mesaj kopyalandı"); }}>Copy</button>{!message ? <p>Henüz tedarikçi mesajı oluşturulmadı</p> : null}</section>
      <section><h3>5. Quote Builder</h3><div className="form-grid">{Object.keys(quoteForm).map((key) => <label key={key}>{key}<input type={key.includes("until") ? "date" : "text"} value={(quoteForm as AnyObj)[key]} onChange={(e) => setQuoteForm({ ...quoteForm, [key]: e.target.value })} /></label>)}</div><button className="btn btn-primary" onClick={() => callApi("/api/owner/quotes/calculate", quoteForm, (d) => setQuoteResult(d))}>Calculate Quote</button>{quoteResult ? <div><p><b>final_customer_quote:</b> {quoteResult.final_customer_price}</p><p><b>estimated_delivery_window:</b> {(Number(quoteForm.production_days || 0) + Number(quoteForm.shipping_days || 0) + Number(quoteForm.customs_days || 0)) || "-"} gün</p><p><b>internal margin:</b> {quoteResult.margin_amount}</p></div> : null}<p className="muted-note">Bu maliyet ve marj bilgileri müşteriye gösterilmez.</p></section>
      <section><h3>6. Escrow</h3><button className="btn btn-primary" disabled={!selectedId || loading === "/api/owner/escrow/prepare"} onClick={() => callApi("/api/owner/escrow/prepare", {}, (d) => setEscrow(d))}>Prepare Escrow</button><p>escrow status: {escrow?.escrow_status || "-"}</p>{escrow?.payment_link ? <a href={escrow.payment_link}>payment link</a> : null}</section>
      <section><h3>7. Milestones</h3><ul>{MILESTONES.map((m) => <li key={m}>{m} {milestones.some((x: AnyObj) => String(x.milestone_name).toLowerCase() === m.toLowerCase()) ? "✅" : "○"}</li>)}</ul></section>
      {status ? <p>{status}</p> : null}
      </article>
    </section>
  </main>;
}
