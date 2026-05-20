"use client";

const machineModels = [
  {
    code: "Fine Cleaner Model 5X-5 (Amiral Gemisi Komple Sistem)",
    details: [
      "Tam entegre tahıl eleme ve hava aspirasyon tesisi; ana 5X-5 gövde bileşeni ($13,000).",
      "Model 4-72-4.5A Emiş Fanı (7.5KW), Model 2-0900 Cyclone Air Locker ($1,800, 1.1KW), entegre invertör kontrollü pano.",
      "Model W6 düşük devir kırılma önleyici kovalı elevatör (1.1KW) ve beyaz fasulye için 1 takım 7PCS özel elek.",
      "Kapasite: buğday yoğunluğu bazında 5 TPH. Ölçüler: 3200x1940x3600 mm."
    ]
  },
  {
    code: "LCSX Intelligent Photoelectric Color Sorter",
    details: [
      "Dijital fotoelektrik hücre matris teknolojisi ile optik sınıflandırma ızgarası.",
      "Şekil tanıma algoritmaları ile akıllı ayıklama.",
      "Hava basıncı için akıllı bulut izleme ve mobil APP üzerinden uzaktan kalibrasyon.",
      "Tekli/çift kanallı genişleme desteği."
    ]
  },
  {
    code: "High-Capacity TQSF Series Gravity De-Stoner",
    details: [
      "Özgül ağırlık farkına göre taş ve ağır yabancı madde ayrımı.",
      "Resiprokal çift katlı elek tablaları ve akışkan hava süspansiyonu kombinasyonu.",
      "Kapalı negatif basınç şasesi ile toz emisyonunu engeller.",
      "Tabla eğimi ve hava hızı bağımsız ayarlanabilir."
    ]
  },
  {
    code: "DCS Electronic Quantitative Packing Scale",
    details: [
      "Mikrobilgisayar kontrollü otomatik torbalama, yüksek hızlı tartım ve bant konveyörlü dikiş istasyonu.",
      "Programlanabilir çalışma aralığı: 10kg - 65kg.",
      "Kapasite: 420-1080 torba/saat.",
      "Hassasiyet toleransı: ±0.2%."
    ]
  },
  {
    code: "5XZ Series Gravity Separator (Özgül Ağırlık Masası)",
    details: [
      "Ağır hizmet tipi hava masalı vibrasyonlu yoğunluk ayırıcı.",
      "Küflü, çimlenmiş, yanık veya böcek hasarlı hafif tanelerin ayrıştırılması için tasarlanmıştır.",
      "Değişken salınım hızı ve çoklu fan hava hacmi yönetimi sunar."
    ]
  },
  {
    code: "5BY Series Automatic Seed Coater (Tohum İlaçlama)",
    details: [
      "Hassas dozaj mikroişlemcileri ile otomatik sıvı santrifüj batch karışım temizleyici.",
      "Senkron çift salınımlı disklerle koruyucu kimyasal kaplamayı homojen atomize eder.",
      "Sürtünmeye bağlı kabuk zararını minimize edecek şekilde tasarlanmıştır."
    ]
  },
  {
    code: "TQLZ Series Vibrating Pre-Cleaner",
    details: [
      "Çift vibrasyon motorlu yüksek hacimli ön ayırıcı.",
      "İnce eleme öncesi iri saman, ip/parça ve yüzey döküntülerini hızlıca uzaklaştırır.",
      "Kolay değiştirilebilir elek kasetleri ile bakım kolaylığı sağlar."
    ]
  },
  {
    code: "DT Series Heavy-Duty Bucket Elevator (Dikey Taşıma)",
    details: [
      "Endüstriyel dökme malzeme için dikey taşıma konveyörü.",
      "Aşınmaya dayanıklı yüksek yoğunluklu polimer kovalar.",
      "Düzgün anti-shred hat hızı ve mekanik geri kaçırmaz fren emniyet kilitleri."
    ]
  }
];

export default function HomePage() {
  const scrollToContact = (productName: string) => {
    const selectElement = document.getElementById("productSelect") as HTMLSelectElement | null;
    if (selectElement) {
      selectElement.value = productName;
      selectElement.dispatchEvent(new Event("change", { bubbles: true }));
    }

    const contactSection = document.getElementById("corporate-rfq-form");
    if (contactSection) {
      contactSection.scrollIntoView({ behavior: "smooth", block: "start" });
    }
  };

  return (
    <main className="mx-auto max-w-7xl px-4 py-10 md:px-8">
      <section className="rounded-2xl bg-slate-900 p-8 text-white md:p-12">
        <p className="text-sm uppercase tracking-[0.2em] text-sky-300">TradePi Globall</p>
        <h1 className="mt-3 text-3xl font-bold md:text-5xl">Endüstriyel Tahıl & Tohum İşleme RFO Platformu</h1>
        <p className="mt-5 max-w-3xl text-slate-200">
          Tüm teklifler yalnızca kurumsal talep formu (RFO) üzerinden hazırlanır. Ürün konfigürasyonunu seçin,
          saha bilgilerinizi paylaşın, size özel teknik ve lojistik teklif dosyasını oluşturalım.
        </p>
      </section>

      <section className="mt-12">
        <h2 className="text-2xl font-semibold text-slate-900 md:text-3xl">Makine Listesi & Teknik Parametreler</h2>
        <div className="mt-6 grid gap-6 md:grid-cols-2">
          {machineModels.map((machine) => (
            <article key={machine.code} className="rounded-2xl border border-slate-200 bg-white p-6 shadow-sm">
              <h3 className="text-xl font-semibold text-slate-900">{machine.code}</h3>
              <div className="mt-4 grid gap-4 lg:grid-cols-2">
                <img src="" alt={machine.code} className="h-48 w-full rounded-xl object-cover" />
                <div className="h-48 w-full rounded-xl border border-dashed border-slate-300 bg-slate-50 p-4">
                  <p className="text-sm text-slate-500">Video alanı: YouTube iframe embed için hazır konteyner.</p>
                </div>
              </div>
              <ul className="mt-4 list-disc space-y-2 pl-5 text-sm text-slate-700">
                {machine.details.map((item) => (
                  <li key={item}>{item}</li>
                ))}
              </ul>
              <button
                type="button"
                onClick={() => scrollToContact(machine.code)}
                className="mt-5 rounded-xl bg-slate-900 px-4 py-2 text-sm font-medium text-white transition hover:bg-slate-700"
              >
                Bu Model İçin RFO Gönder
              </button>
            </article>
          ))}
        </div>
      </section>

      <section id="corporate-rfq-form" className="mt-14 rounded-2xl border border-slate-200 bg-white p-6 md:p-8">
        <h2 className="text-2xl font-semibold text-slate-900">Kurumsal Veri Toplama Formu</h2>
        <p className="mt-3 text-sm text-slate-600">
          Kurulum yapılacak ili ve mahsul tipinizi belirtin, adınıza özel güncel DDP lojistik ve gümrük teklifini
          hazırlayalım.
        </p>

        <form className="mt-6 grid gap-4 md:grid-cols-2">
          <label className="flex flex-col gap-2 text-sm font-medium text-slate-700 md:col-span-2">
            Model Seçimi
            <select id="productSelect" name="productSelect" className="rounded-xl border border-slate-300 px-3 py-2">
              <option value="">Model seçiniz</option>
              {machineModels.map((machine) => (
                <option key={machine.code} value={machine.code}>
                  {machine.code}
                </option>
              ))}
            </select>
          </label>

          <label className="flex flex-col gap-2 text-sm font-medium text-slate-700">
            Firma Adı
            <input type="text" name="companyName" className="rounded-xl border border-slate-300 px-3 py-2" />
          </label>

          <label className="flex flex-col gap-2 text-sm font-medium text-slate-700">
            Yetkili Kişi
            <input type="text" name="contactName" className="rounded-xl border border-slate-300 px-3 py-2" />
          </label>

          <label className="flex flex-col gap-2 text-sm font-medium text-slate-700">
            Kurulum İli
            <input type="text" name="installationCity" className="rounded-xl border border-slate-300 px-3 py-2" />
          </label>

          <label className="flex flex-col gap-2 text-sm font-medium text-slate-700">
            Mahsul Tipi
            <input type="text" name="cropType" className="rounded-xl border border-slate-300 px-3 py-2" />
          </label>

          <label className="flex flex-col gap-2 text-sm font-medium text-slate-700 md:col-span-2">
            İhtiyaç Notları
            <textarea name="requirements" rows={4} className="rounded-xl border border-slate-300 px-3 py-2" />
          </label>

          <button type="submit" className="rounded-xl bg-sky-700 px-5 py-2.5 text-sm font-semibold text-white md:col-span-2">
            RFO Talebini İlet
          </button>
        </form>
      </section>
    </main>
  );
}
