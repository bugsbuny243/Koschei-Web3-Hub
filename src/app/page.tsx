import { Button } from "@/components/Button";
import { Card } from "@/components/Card";
import { Header } from "@/components/Header";
import { PricingCard } from "@/components/PricingCard";

const steps = [
  ["01", "Talebi girin", "Firma, alıcı ve ürün bilgilerini sade form üzerinden birkaç dakikada doldurun."],
  ["02", "Teklifiniz hazırlansın", "TeklifPilot profesyonel İngilizce teklif metnini ve takip mesajını oluştursun."],
  ["03", "PDF olarak paylaşın", "Temiz teklif sayfasını yazdırın, PDF olarak kaydedin ve müşterinize iletin."],
];

export default function Home() {
  return (
    <main>
      <Header />
      <section className="overflow-hidden bg-slate-950 text-white">
        <div className="mx-auto grid max-w-7xl gap-14 px-5 py-20 lg:grid-cols-[1.05fr_.95fr] lg:px-8 lg:py-28">
          <div>
            <span className="rounded-full border border-cyan-400/30 bg-cyan-400/10 px-4 py-2 text-xs font-bold tracking-widest text-cyan-300">KOBİ&apos;LER İÇİN İHRACAT ASİSTANI</span>
            <h1 className="mt-7 max-w-3xl text-4xl font-black leading-tight tracking-tight sm:text-6xl">WhatsApp&apos;tan gelen ürün taleplerini <span className="text-cyan-400">5 dakikada</span> İngilizce ihracat teklifine çevir.</h1>
            <p className="mt-6 max-w-2xl text-lg leading-8 text-slate-300">Dağınık mesajlardan profesyonel tekliflere geçin. TeklifPilot, ihracat teklifinizi hazırlar; siz satışa ve müşterinize odaklanırsınız.</p>
            <div className="mt-9 flex flex-wrap gap-3"><Button href="/quote/new">Teklif Oluştur <span className="ml-1">→</span></Button><Button href="/dashboard" variant="secondary">Panele Git</Button></div>
            <div className="mt-9 flex flex-wrap gap-x-6 gap-y-2 text-sm text-slate-400"><span>✓ Kredi kartı gerekmez</span><span>✓ Kurulum gerekmez</span><span>✓ PDF çıktısı hazır</span></div>
          </div>
          <div className="relative hidden items-center lg:flex">
            <div className="absolute inset-8 rounded-full bg-cyan-400/20 blur-3xl" />
            <Card className="relative w-full overflow-hidden border-slate-700 bg-white p-3 text-slate-900 shadow-2xl">
              <div className="rounded-xl bg-slate-50 p-7">
                <div className="flex items-start justify-between border-b border-slate-200 pb-5"><div><p className="text-xl font-black">ACME EXPORT</p><p className="mt-1 text-xs text-slate-500">Professional export solutions</p></div><span className="rounded-lg bg-cyan-100 px-3 py-2 text-xs font-black text-cyan-800">QUOTATION</span></div>
                <div className="grid grid-cols-2 gap-5 py-5 text-xs"><div><p className="font-bold text-slate-400">PREPARED FOR</p><p className="mt-2 font-bold">Nordic Trade GmbH</p><p className="mt-1 text-slate-500">Germany</p></div><div><p className="font-bold text-slate-400">QUOTE NUMBER</p><p className="mt-2 font-bold">TP-20260530-X7A2</p></div></div>
                <div className="rounded-lg bg-white p-4 text-xs shadow-sm"><div className="grid grid-cols-4 border-b border-slate-100 pb-3 font-bold text-slate-400"><span className="col-span-2">PRODUCT</span><span>QTY</span><span>TOTAL</span></div><div className="grid grid-cols-4 pt-3 font-bold"><span className="col-span-2">Premium Ceramic Set</span><span>250 pcs</span><span>$3,750</span></div></div>
                <div className="mt-5 flex justify-between text-sm font-black"><span>Total Amount</span><span className="text-cyan-700">$3,750.00</span></div>
              </div>
            </Card>
          </div>
        </div>
      </section>

      <section className="mx-auto max-w-7xl px-5 py-20 lg:px-8">
        <div className="grid gap-7 lg:grid-cols-2">
          <Card className="bg-slate-950 p-8 text-white"><p className="text-xs font-black tracking-widest text-cyan-400">SORUN</p><h2 className="mt-4 text-3xl font-black">İhracat fırsatları dağınık mesajlarda kaybolmasın.</h2><p className="mt-4 leading-7 text-slate-300">WhatsApp&apos;tan gelen ürün soruları, Excel tabloları ve eksik İngilizce metinler satış ekibinizi yavaşlatır. Teklif geciktiğinde müşteri başka tedarikçiye gider.</p></Card>
          <Card className="p-8"><p className="text-xs font-black tracking-widest text-cyan-700">ÇÖZÜM</p><h2 className="mt-4 text-3xl font-black text-slate-950">Dakikalar içinde güven veren bir teklif gönderin.</h2><p className="mt-4 leading-7 text-slate-600">TeklifPilot ürün, fiyat ve teslimat bilgilerinizi tek sayfada toplar; İngilizce teklif metnini, takip mesajını ve yazdırılabilir teklif sayfasını otomatik hazırlar.</p></Card>
        </div>
      </section>

      <section className="bg-slate-100 py-20">
        <div className="mx-auto max-w-7xl px-5 lg:px-8"><p className="text-xs font-black tracking-widest text-cyan-700">NASIL ÇALIŞIR?</p><h2 className="mt-3 text-3xl font-black text-slate-950 sm:text-4xl">İlk teklifiniz üç adımda hazır.</h2><div className="mt-10 grid gap-5 md:grid-cols-3">{steps.map(([number, title, copy]) => <Card key={number} className="p-7"><span className="text-sm font-black text-cyan-600">{number}</span><h3 className="mt-4 text-xl font-black">{title}</h3><p className="mt-3 text-sm leading-6 text-slate-600">{copy}</p></Card>)}</div></div>
      </section>

      <section className="mx-auto max-w-7xl px-5 py-20 lg:px-8" id="pricing"><div className="mx-auto max-w-2xl text-center"><p className="text-xs font-black tracking-widest text-cyan-700">FİYATLANDIRMA</p><h2 className="mt-3 text-3xl font-black sm:text-4xl">İhracat hızınıza uygun planı seçin.</h2><p className="mt-4 leading-7 text-slate-600">Bugün ücretsiz MVP ile deneyin. İşletmenize özel kurulum için size en uygun paketle başlayın.</p></div><div className="mt-10 grid gap-5 lg:grid-cols-3"><PricingCard name="Başlangıç" price="2.500 TL kurulum + 1.499 TL/ay" description="İlk ihracat tekliflerini hızlandırmak isteyen işletmeler için." features={["Profesyonel teklif sayfası", "İngilizce teklif metni", "PDF olarak dışa aktarma"]}/><PricingCard name="Pro" price="4.999 TL/ay" description="Düzenli teklif gönderen büyüyen satış ekipleri için." features={["Başlangıç özelliklerinin tamamı", "Ekip kullanımına hazır yapı", "Öncelikli destek"]} featured/><PricingCard name="Ajans/İhracatçı" price="9.999 TL/ay" description="Birden fazla marka ve müşteri yöneten ekipler için." features={["Pro özelliklerinin tamamı", "Çoklu firma senaryoları", "Özel süreç danışmanlığı"]}/></div></section>

      <section className="bg-cyan-400"><div className="mx-auto flex max-w-7xl flex-col items-start justify-between gap-5 px-5 py-12 lg:flex-row lg:items-center lg:px-8"><div><h2 className="text-3xl font-black text-slate-950">İlk teklifinizi bugün hazırlayın.</h2><p className="mt-2 text-slate-800">Formu doldurun, teklifinizi kontrol edin ve PDF olarak müşterinize gönderin.</p></div><Button href="/quote/new" className="bg-slate-950 text-white hover:bg-slate-800">Teklif Oluştur <span className="ml-1">→</span></Button></div></section>
      <footer className="bg-slate-950 px-5 py-8 text-center text-sm text-slate-400">© 2026 TeklifPilot. KOBİ&apos;ler için hızlı ve profesyonel ihracat teklifleri.</footer>
    </main>
  );
}
