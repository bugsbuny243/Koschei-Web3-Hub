"use client";

const machineModels = [
  {
    name: "Fine Cleaner Model 5X-5 (Amiral Gemisi Komple Sistem)",
    details: [
      "Tam entegre tahıl eleme ve hava aspirasyon tesisi; ana 5X-5 gövdesi (komponent değeri: $13,000).",
      "Model 4-72-4.5A Emiş Fanı: 7.5KW.",
      "Model 2-0900 Cyclone Air Locker: $1,800, 1.1KW.",
      "İnverter kontrollü entegre kontrol kabini.",
      "Model W6 düşük hızlı kırılma önleyici kovalı elevatör: 1.1KW.",
      "Beyaz fasulye işleme için 1 takım 7 adet özel elek.",
      "Kapasite: Buğday yoğunluğu bazında 5 TPH.",
      "Ölçüler: 3200x1940x3600 mm.",
    ],
  },
  {
    name: "LCSX Intelligent Photoelectric Color Sorter",
    details: [
      "Dijital fotoelektrik hücre matris teknolojisiyle optik ayıklama ızgarası.",
      "Şekil tanıma algoritmalarıyla akıllı ayıklama.",
      "Hava basıncı için akıllı bulut izleme.",
      "Mobil APP arayüzü üzerinden uzaktan kalibrasyon.",
      "Tekli/çift kanallı genişleme desteği.",
    ],
  },
  {
    name: "High-Capacity TQSF Series Gravity De-Stoner",
    details: [
      "Özgül ağırlık farkına göre ağır yabancı madde ve taş ayırma.",
      "Karşılıklı hareketli çift katmanlı elek + akışkan hava süspansiyonu.",
      "Toz emisyonunu engelleyen kapalı negatif basınçlı şase.",
      "Elek eğimi ve hava hızı bağımsız ayarlanabilir.",
    ],
  },
  {
    name: "DCS Electronic Quantitative Packing Scale",
    details: [
      "Mikrobilgisayar kontrollü otomatik torbalama.",
      "Yüksek hızlı tartım ve bant konveyör dikiş istasyonu.",
      "Programlanabilir çalışma aralığı: 10kg - 65kg.",
      "Kapasite: 420-1080 torba/saat.",
      "Hassasiyet toleransı: ±0.2%.",
    ],
  },
  {
    name: "5XZ Series Gravity Separator (Özgül Ağırlık Masası)",
    details: [
      "Ağır hizmet tipi hava masalı titreşimli yoğunluk separatörü.",
      "Küflü, çimlenmiş, bozulmuş veya böcek hasarlı hafif taneleri uzaklaştırır.",
      "Değişken salınım hızı ve çoklu fan hava hacmi yönetimi.",
    ],
  },
  {
    name: "5BY Series Automatic Seed Coater (Tohum İlaçlama)",
    details: [
      "Hassas dozaj mikroişlemcileriyle otomatik sıvı santrifüj batch karıştırma.",
      "Senkron çift salınımlı disklerle kimyasal kaplamayı homojen atomize eder.",
      "Tohum kabuğunda sürtünme kaynaklı hasarı azaltan uygulama mimarisi.",
    ],
  },
  {
    name: "TQLZ Series Vibrating Pre-Cleaner",
    details: [
      "Çift titreşim motorlu yüksek hacimli ön temizleme separatörü.",
      "İnce eleme öncesi kaba sap, ip ve yüzey atıklarını hızlıca uzaklaştırır.",
      "Kolay değiştirilebilir elek kaset yapısı.",
    ],
  },
  {
    name: "DT Series Heavy-Duty Bucket Elevator (Dikey Taşıma)",
    details: [
      "Endüstriyel dikey dökme malzeme taşıma konveyörü.",
      "Aşınmaya dayanıklı yüksek yoğunluklu polimer kova seti.",
      "Düzgün anti-shred hat hızı.",
      "Mekanik geri kaçırmaz fren güvenlik kilitleri.",
    ],
  },
];

export default function HomePage() {
  const scrollToContact = (productName: string) => {
    const select = document.getElementById("productSelect") as HTMLSelectElement | null;
    if (select) {
      select.value = productName;
      select.dispatchEvent(new Event("change", { bubbles: true }));
    }

    const formSection = document.getElementById("corporate-rfo-form");
    if (formSection) {
      formSection.scrollIntoView({ behavior: "smooth", block: "start" });
    }
  };

  return (
    <main className="mx-auto max-w-7xl space-y-10 p-6 md:p-10">
      <section className="rounded-2xl bg-slate-900 p-8 text-white">
        <p className="text-sm uppercase tracking-[0.2em]">TradePi Globall</p>
        <h1 className="mt-2 text-3xl font-bold md:text-4xl">Kurumsal Endüstriyel Hatlar - Request For Quote (RFO)</h1>
        <p className="mt-4 text-slate-200">
          Bu sayfa yalnızca kurumsal teklif toplama amaçlıdır. Teknik uygunluk doğrulaması sonrası
          ürün bazlı resmi teklif hazırlanır.
        </p>
      </section>

      <section className="space-y-6">
        <h2 className="text-2xl font-semibold">Makine Listesi ve Teknik Parametreler</h2>
        <div className="grid gap-6 md:grid-cols-2">
          {machineModels.map((machine) => (
            <article key={machine.name} className="rounded-2xl border border-slate-200 bg-white p-5 shadow-sm">
              <img src="" alt={machine.name} className="h-48 w-full rounded-xl object-cover" />
              <div className="mt-4 rounded-xl border border-dashed border-slate-300 bg-slate-50 p-4">
                <p className="text-sm font-medium text-slate-600">Video Alanı (YouTube iframe hazır)</p>
                <div className="mt-2 aspect-video w-full rounded-lg bg-slate-200" />
              </div>
              <h3 className="mt-4 text-lg font-semibold">{machine.name}</h3>
              <ul className="mt-3 list-disc space-y-2 pl-5 text-sm text-slate-700">
                {machine.details.map((detail) => (
                  <li key={detail}>{detail}</li>
                ))}
              </ul>
              <button
                type="button"
                onClick={() => scrollToContact(machine.name)}
                className="mt-5 rounded-lg bg-slate-900 px-4 py-2 text-sm font-medium text-white hover:bg-slate-800"
              >
                Bu model için teklif iste
              </button>
            </article>
          ))}
        </div>
      </section>

      <section id="corporate-rfo-form" className="rounded-2xl border border-slate-200 bg-white p-6 shadow-sm">
        <h2 className="text-2xl font-semibold">Kurumsal Veri Alım Formu</h2>
        <p className="mt-2 text-sm text-slate-700">
          Kurulum yapılacak ili ve mahsul tipinizi belirtin, adınıza özel güncel DDP lojistik ve gümrük teklifini hazırlayalım.
        </p>
        <form className="mt-6 grid gap-4 md:grid-cols-2">
          <label className="flex flex-col gap-2 text-sm font-medium text-slate-700 md:col-span-2">
            Talep edilen model
            <select id="productSelect" name="productSelect" className="rounded-lg border border-slate-300 p-3">
              <option value="">Model seçiniz</option>
              {machineModels.map((machine) => (
                <option key={machine.name} value={machine.name}>
                  {machine.name}
                </option>
              ))}
            </select>
          </label>
          <label className="flex flex-col gap-2 text-sm font-medium text-slate-700">
            Firma Adı
            <input type="text" name="company" className="rounded-lg border border-slate-300 p-3" />
          </label>
          <label className="flex flex-col gap-2 text-sm font-medium text-slate-700">
            İletişim Kişisi
            <input type="text" name="contact" className="rounded-lg border border-slate-300 p-3" />
          </label>
          <label className="flex flex-col gap-2 text-sm font-medium text-slate-700">
            Kurulum İli
            <input type="text" name="city" className="rounded-lg border border-slate-300 p-3" />
          </label>
          <label className="flex flex-col gap-2 text-sm font-medium text-slate-700">
            Mahsul Tipi
            <input type="text" name="crop" className="rounded-lg border border-slate-300 p-3" />
          </label>
          <button type="submit" className="rounded-lg bg-slate-900 px-4 py-3 text-sm font-medium text-white md:col-span-2">
            Kurumsal RFO Gönder
          </button>
        </form>
      </section>
    </main>
  );
}
