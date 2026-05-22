"use client";

import Link from "next/link";
import { useMemo, useState } from "react";

type AnyObj = Record<string, any>;

const MILESTONES = [
  "Talep alındı",
  "AI analizi yapıldı",
  "Tedarikçiyle iletişime geçildi",
  "Tedarikçi teklifi alındı",
  "Müşteri teklifi hazırlandı",
  "Müşteri onayı alındı",
  "Escrow oluşturuldu",
  "Ödeme güvenceye alındı",
  "Üretim başladı",
  "Sevkiyat başladı",
  "Gümrük süreci",
  "Teslim edildi",
];

const QUOTE_LABELS: Record<string, string> = {
  supplier_machine_cost: "Tedarikçi makine maliyeti",
  supplier_ddp_total_cost: "Tedarikçi DDP toplam maliyeti",
  tradepi_margin_type: "Kar marjı tipi",
  tradepi_margin_value: "Kar marjı değeri",
  escrow_fee_estimate: "Escrow ücret tahmini",
  production_days: "Üretim süresi",
  shipping_days: "Nakliye süresi",
  customs_days: "Gümrük süresi",
  quote_valid_until: "Teklif geçerlilik tarihi",
};

function trError(error: string) {
  if (error.includes("missing") || error.includes("BRAVE_SEARCH_API_KEY")) return "API yapılandırması eksik";
  if (error.includes("unauthorized")) return "Yetkisiz işlem";
  if (error.includes("rfq not found")) return "Talep bulunamadı";
  return "İşlem sırasında bir hata oluştu";
}

export function CommandCenterClient({ fallbackPassword, initialRfqs, initialSelectedId, initialAnalyses, initialMarket, initialMessages, initialEscrow, initialMilestones }: AnyObj) {
  const [selectedId, setSelectedId] = useState<string | null>(initialSelectedId);
  const [analyses, setAnalyses] = useState(initialAnalyses || []);
  const [market, setMarket] = useState(initialMarket || []);
  const [message, setMessage] = useState(initialMessages?.[0]?.message_json ? JSON.stringify(initialMessages[0].message_json, null, 2) : "");
  const [escrow, setEscrow] = useState(initialEscrow);
  const [milestones] = useState(initialMilestones || []);
  const [status, setStatus] = useState("");
  const [loading, setLoading] = useState<string | null>(null);
  const selected = useMemo(() => initialRfqs.find((x: AnyObj) => x.id === selectedId), [initialRfqs, selectedId]);
  const [quoteForm, setQuoteForm] = useState({ supplier_machine_cost: "", supplier_ddp_total_cost: "", tradepi_margin_type: "percent", tradepi_margin_value: "", escrow_fee_estimate: "", production_days: "", shipping_days: "", customs_days: "", quote_valid_until: "" });
  const [quoteResult, setQuoteResult] = useState<AnyObj | null>(null);
  const noSelectionMessage = "Önce bir teklif talebi seçmelisin.";

  const counters = {
    total: initialRfqs.length,
    waitingSupplierQuote: initialRfqs.filter((r: AnyObj) => String(r.status || "").toLowerCase().includes("supplier")).length,
    waitingCustomerApproval: initialRfqs.filter((r: AnyObj) => String(r.status || "").toLowerCase().includes("approval")).length,
    escrowPending: initialRfqs.filter((r: AnyObj) => String(r.status || "").toLowerCase().includes("escrow")).length,
    inProduction: initialRfqs.filter((r: AnyObj) => String(r.status || "").toLowerCase().includes("production")).length,
    delivered: initialRfqs.filter((r: AnyObj) => String(r.status || "").toLowerCase().includes("delivered")).length,
  };

  async function callApi(path: string, body: AnyObj, setter?: (data: AnyObj) => void) {
    if (!selectedId) return setStatus(noSelectionMessage);
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

  const markedMilestones = MILESTONES.map((label, index) => {
    const isDone = milestones.length > index;
    const isActive = !isDone && !!selectedId && milestones.length === index;
    return { label, state: isDone ? "done" : isActive ? "active" : "pending" };
  });

  return <main className="owner-dashboard">
    <section className="owner-top">
      <div><h1>TradePi Owner Komuta Merkezi</h1><p>Teklif talepleri, tedarikçi süreçleri, escrow ve teslimat operasyonları</p></div>
      <div className="owner-quick-actions"><Link className="btn btn-secondary" href="#rfq-inbox">Teklif Talepleri</Link><Link className="btn btn-secondary" href="/owner/supplier-outreach">Tedarikçi Arama</Link><Link className="btn btn-secondary" href="/owner/media-library">Medya Kütüphanesi</Link><button className="btn btn-primary" type="button" disabled={!selectedId}>Yeni Teklif</button></div>
    </section>

    <section className="owner-kpis">{[["Toplam Talep", counters.total], ["Tedarikçi Teklifi Bekliyor", counters.waitingSupplierQuote], ["Müşteri Onayı Bekliyor", counters.waitingCustomerApproval], ["Escrow Bekliyor", counters.escrowPending], ["Üretimde", counters.inProduction], ["Teslim Edildi", counters.delivered]].map(([k, v]) => <article key={String(k)} className="owner-kpi-card"><p>{k}</p><strong>{String(v)}</strong></article>)}</section>

    <section className="owner-main-grid">
      <article className="card" id="rfq-inbox"><h2>Teklif Talepleri</h2>{initialRfqs.length === 0 ? <div className="owner-empty-state"><p>Henüz teklif talebi yok. Müşteriler Teklif Al formunu doldurduğunda burada görünecek.</p><Link className="btn btn-primary" href="/request-quote">Public Teklif Formunu Aç</Link></div> : <div className="owner-rfq-list">{initialRfqs.map((rfq: AnyObj) => <div key={rfq.id} className="owner-rfq-row"><div><strong>{rfq.company_name || rfq.full_name}</strong><p>{rfq.city} • {rfq.product_interest}</p><p>Kapasite: {rfq.required_capacity_tph ?? "-"}</p></div><div><span className="badge">{rfq.status || "open"}</span><p>{new Date(rfq.created_at).toLocaleDateString("tr-TR")}</p><button className="btn btn-secondary" type="button" onClick={() => setSelectedId(rfq.id)}>Seç</button></div></div>)}</div>}</article>

      <article className="card"><h2>Seçili Talep Çalışma Alanı</h2>
      {!selectedId ? <p className="owner-inline-warning">{noSelectionMessage}</p> : null}
      <section><h3>1. Müşteri Talebi</h3>{selected ? <div className="grid"><p><b>Firma:</b> {selected.company_name || "-"}</p><p><b>İletişim:</b> {selected.full_name || "-"} / {selected.email || "-"}</p><p><b>Şehir:</b> {selected.city || "-"}</p><p><b>Ürün:</b> {selected.crop || selected.product_interest || "-"}</p><p><b>Kapasite:</b> {selected.required_capacity_tph || "-"}</p><p><b>Teslimat Adresi:</b> {selected.delivery_address || "-"}</p><p><b>Notlar:</b> {selected.notes || "-"}</p></div> : <p>{noSelectionMessage}</p>}</section>
      <section><h3>2. AI Analizi</h3><button className="btn btn-primary" disabled={!selectedId || loading === "/api/owner/ai/analyze-rfq"} onClick={() => callApi("/api/owner/ai/analyze-rfq", {}, (d) => setAnalyses([{ analysis_json: d.analysis }, ...analyses]))}>{loading === "/api/owner/ai/analyze-rfq" ? "Yükleniyor..." : "AI ile Analiz Et"}</button>{analyses.length ? <pre>{JSON.stringify(analyses[0].analysis_json, null, 2)}</pre> : <p>Henüz AI analizi yok</p>}</section>
      <section><h3>3. Piyasa Araştırması</h3><button className="btn btn-primary" disabled={!selectedId || loading === "/api/owner/ai/market-research"} onClick={() => callApi("/api/owner/ai/market-research", { query: `${selected?.product_interest || "machinery"} suppliers Turkey` }, (d) => setMarket((d.sources || []).map((s: AnyObj) => ({ source_url: s.url }))))}>Piyasa Araştırması Çalıştır</button>{market.length ? <ul>{market.map((m: AnyObj, i: number) => <li key={i}><a href={m.source_url} target="_blank" rel="noreferrer">{m.source_url}</a></li>)}</ul> : <p>Henüz piyasa araştırması yok</p>}</section>
      <section><h3>4. Tedarikçi Mesajı</h3><button className="btn btn-primary" disabled={!selectedId || loading === "/api/owner/ai/generate-supplier-message"} onClick={() => callApi("/api/owner/ai/generate-supplier-message", {}, (d) => setMessage(JSON.stringify(d.message, null, 2)))}>Tedarikçi Mesajı Oluştur</button><textarea value={message} onChange={(e) => setMessage(e.target.value)} rows={6} /> <button className="btn btn-secondary" type="button" onClick={async () => { await navigator.clipboard.writeText(message); setStatus("Mesaj kopyalandı"); }}>Kopyala</button>{!message ? <p>Henüz tedarikçi mesajı oluşturulmadı</p> : null}</section>
      <section><h3>5. Teklif Hesaplayıcı</h3><div className="form-grid owner-quote-grid">{Object.keys(quoteForm).map((key) => <label key={key}>{QUOTE_LABELS[key] || key}<input type={key.includes("until") ? "date" : "text"} value={(quoteForm as AnyObj)[key]} onChange={(e) => setQuoteForm({ ...quoteForm, [key]: e.target.value })} /></label>)}</div><button className="btn btn-primary" disabled={!selectedId} onClick={() => callApi("/api/owner/quotes/calculate", quoteForm, (d) => setQuoteResult(d))}>Teklifi Hesapla</button>{quoteResult ? <div><p><b>Nihai müşteri teklifi:</b> {quoteResult.final_customer_price}</p><p><b>Tahmini teslim süresi:</b> {(Number(quoteForm.production_days || 0) + Number(quoteForm.shipping_days || 0) + Number(quoteForm.customs_days || 0)) || "-"} gün</p><p><b>İç marj:</b> {quoteResult.margin_amount}</p></div> : null}<p className="muted-note">Bu maliyet ve marj bilgileri müşteriye gösterilmez.</p></section>
      <section><h3>6. Güvenli Ödeme Hazırlığı</h3><button className="btn btn-primary" disabled={!selectedId || loading === "/api/owner/escrow/prepare"} onClick={() => callApi("/api/owner/escrow/prepare", {}, (d) => setEscrow(d))}>Escrow Hazırla</button><p>Escrow durumu: {escrow?.escrow_status || "-"}</p>{escrow?.payment_link ? <a href={escrow.payment_link}>Ödeme bağlantısı</a> : null}</section>
      <section><h3>7. Süreç Takibi</h3><div className="owner-timeline">{markedMilestones.map((m) => <div key={m.label} className={`owner-timeline-item ${m.state}`}><span className="owner-dot" /><p>{m.label}</p></div>)}</div></section>
      {status ? <p>{status}</p> : null}
      </article>
    </section>
  </main>;
}
