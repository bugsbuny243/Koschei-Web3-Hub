import { Header } from "@/components/Header";
import { QuoteForm } from "@/components/QuoteForm";

export default function NewQuotePage() { return <main className="min-h-screen bg-slate-50"><Header/><div className="mx-auto max-w-4xl px-5 py-10"><p className="text-xs font-black tracking-widest text-cyan-700">YENİ TEKLİF</p><h1 className="mt-2 text-3xl font-black">Talebi profesyonel bir teklife dönüştürün.</h1><p className="mt-3 max-w-2xl leading-7 text-slate-600">Temel bilgileri girin. TeklifPilot İngilizce teklif metnini, takip mesajını ve yazdırılabilir teklif sayfasını hazırlasın.</p><div className="mt-8"><QuoteForm/></div></div></main>; }
