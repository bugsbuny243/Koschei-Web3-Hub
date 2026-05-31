"use client";

import { useEffect, useState } from "react";
import { Button } from "@/components/Button";
import { PrintableQuote } from "@/components/PrintableQuote";
import { getLatestQuoteFromLocalStorage } from "@/lib/quote";
import type { QuoteData } from "@/lib/types";

export default function PreviewPage() {
  const [quote, setQuote] = useState<QuoteData | null | undefined>(undefined);
  const [copied, setCopied] = useState(false);
  useEffect(() => {
    const timeout = window.setTimeout(() => setQuote(getLatestQuoteFromLocalStorage()), 0);
    return () => window.clearTimeout(timeout);
  }, []);
  if (quote === undefined) return <main className="grid min-h-screen place-items-center bg-slate-100 text-sm font-bold text-slate-500">Teklif yükleniyor...</main>;
  if (!quote) return <main className="grid min-h-screen place-items-center bg-slate-100 px-5 text-center"><div><h1 className="text-2xl font-black">Henüz görüntülenecek bir teklif yok.</h1><p className="mt-2 text-slate-600">Önce müşteri ve ürün bilgilerini girerek teklif oluşturun.</p><Button href="/quote/new" className="mt-6">İlk Teklifi Oluştur</Button></div></main>;
  async function copyMessage() {
    try {
      await navigator.clipboard.writeText(quote!.followUpMessage);
    } catch {
      const textarea = document.createElement("textarea");
      textarea.value = quote!.followUpMessage;
      document.body.appendChild(textarea);
      textarea.select();
      document.execCommand("copy");
      textarea.remove();
    }
    setCopied(true);
    window.setTimeout(() => setCopied(false), 2000);
  }
  return <main className="print-shell min-h-screen bg-slate-100 py-7"><div className="no-print mx-auto mb-5 flex max-w-[800px] flex-col justify-between gap-3 px-4 sm:flex-row sm:items-center"><div><p className="text-xs font-black tracking-widest text-cyan-700">TEKLİF HAZIR</p><h1 className="mt-1 text-xl font-black">Kontrol edin ve müşterinize gönderin.</h1><p className={`mt-2 inline-flex rounded-full px-3 py-1 text-xs font-black ${quote.usedFallback ? "bg-amber-100 text-amber-800" : "bg-emerald-100 text-emerald-800"}`}>{quote.usedFallback ? "Güvenli şablon kullanıldı" : "AI ile oluşturuldu"}</p></div><div className="flex flex-wrap gap-2"><Button href="/quote/new?edit=latest" variant="secondary" className="min-h-11 px-4 py-2">← Düzenle</Button><Button onClick={() => window.print()} className="min-h-11 px-4 py-2">Yazdır / PDF Kaydet</Button></div></div><PrintableQuote quote={quote}/><section className="no-print mx-auto mt-5 max-w-[800px] px-4"><div className="rounded-xl border border-slate-200 bg-white p-5 shadow-sm"><div className="flex flex-col justify-between gap-3 sm:flex-row sm:items-center"><div><p className="text-xs font-black tracking-widest text-cyan-700">TAKİP MESAJI</p><h2 className="mt-1 font-black">WhatsApp veya e-posta için hazır mesaj</h2></div><Button onClick={copyMessage} variant="secondary" className="min-h-11 px-4 py-2">{copied ? "Kopyalandı ✓" : "Mesajı Kopyala"}</Button></div><p className="mt-4 rounded-lg bg-slate-50 p-4 text-sm leading-6 text-slate-600">{quote.followUpMessage}</p></div><div className="mt-4 flex justify-end"><Button href="/quote/new" variant="secondary">+ Yeni Teklif Oluştur</Button></div></section></main>;
}
