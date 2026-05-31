"use client";

import { useEffect, useState } from "react";
import { Button } from "@/components/Button";
import { Card } from "@/components/Card";
import { Header } from "@/components/Header";
import { getQuoteHistory } from "@/lib/quote";
import type { QuoteData } from "@/lib/types";

export default function DashboardPage() {
  const [quotes, setQuotes] = useState<QuoteData[]>([]);
  useEffect(() => setQuotes(getQuoteHistory()), []);
  const latest = quotes[0];

  return (
    <main className="min-h-screen bg-slate-50">
      <Header />
      <div className="mx-auto max-w-7xl px-5 py-10 lg:px-8">
        <div className="flex flex-col justify-between gap-4 sm:flex-row sm:items-end">
          <div><p className="text-xs font-black tracking-widest text-cyan-700">SATIŞ PANELİ</p><h1 className="mt-2 text-3xl font-black tracking-tight text-slate-950">Tekliflerinizi hızla yönetin.</h1><p className="mt-2 text-slate-600">Yeni bir ihracat talebini profesyonel bir teklife dönüştürün.</p></div>
          <Button href="/quote/new">+ Yeni Teklif Oluştur</Button>
        </div>
        <div className="mt-8 grid gap-5 md:grid-cols-3">
          <Card className="p-6"><p className="text-sm font-bold text-slate-500">Toplam teklif</p><p className="mt-3 text-4xl font-black">{quotes.length}</p><p className="mt-2 text-xs text-slate-400">Bu tarayıcıda oluşturulan teklifler</p></Card>
          <Card className="p-6"><p className="text-sm font-bold text-slate-500">Son teklif</p><p className="mt-3 text-xl font-black">{latest?.buyer.company ?? "Henüz teklif yok"}</p><p className="mt-2 text-xs text-slate-400">{latest ? `${latest.quotationNumber} · ${new Date(latest.createdAt).toLocaleDateString("tr-TR")}` : "İlk teklifinizi oluşturmak için başlayın."}</p></Card>
          <Card className="border-cyan-200 bg-cyan-50 p-6"><p className="text-sm font-bold text-cyan-800">Gelir hedefi</p><p className="mt-3 text-3xl font-black text-slate-950">Günlük hedef: 50$</p><p className="mt-2 text-xs text-cyan-800">Her gün en az bir nitelikli teklifi takip edin.</p></Card>
        </div>
        <Card className="mt-7 overflow-hidden"><div className="border-b border-slate-100 p-6"><h2 className="text-lg font-black">Hızlı başlangıç</h2><p className="mt-1 text-sm text-slate-500">Müşterinizden gelen talebi alın ve ilk taslağı dakikalar içinde paylaşın.</p></div><div className="grid gap-5 p-6 md:grid-cols-3">{[["1", "Bilgileri girin", "Firma, müşteri ve ürün detaylarını forma ekleyin."], ["2", "Teklifi kontrol edin", "İngilizce metinleri ve ticari koşulları doğrulayın."], ["3", "PDF paylaşın", "Tarayıcınızdan PDF olarak kaydedip müşteriye iletin."]].map(([n,t,c]) => <div key={n} className="flex gap-3"><span className="flex h-8 w-8 shrink-0 items-center justify-center rounded-lg bg-slate-950 text-sm font-black text-cyan-400">{n}</span><div><h3 className="font-bold">{t}</h3><p className="mt-1 text-sm leading-6 text-slate-500">{c}</p></div></div>)}</div></Card>
        <div className="mt-7 rounded-xl border border-amber-200 bg-amber-50 px-5 py-4 text-sm leading-6 text-amber-900"><strong>MVP notu:</strong> Teklifleriniz şimdilik yalnızca bu tarayıcının yerel depolama alanında saklanır. Tarayıcı verilerini temizlerseniz kayıtlar silinir. Paylaşmadan önce ticari bilgileri kontrol edin.</div>
      </div>
    </main>
  );
}
