import Link from "next/link";

const featuredProducts = [
  { id: 1, name: "Akıllı LED Gece Lambası", price: "₺499", badge: "Çok Satan" },
  { id: 2, name: "Kablosuz Araç Süpürgesi", price: "₺899", badge: "Yeni" },
  { id: 3, name: "Mini Blender Bottle", price: "₺649", badge: "Hızlı Kargo" },
];

export default function HomePage() {
  return (
    <div className="page-stack">
      <section className="hero">
        <p className="eyebrow">Tek Tıkla Trend Ürünler</p>
        <h1>Türkiye&apos;nin yeni nesil dropshipping mağazası</h1>
        <p>
          Viral ürünleri stok riski olmadan sana getiriyoruz. Güvenli ödeme, hızlı teslimat ve
          7/24 destek ile alışveriş artık daha kolay.
        </p>
        <div className="hero-actions">
          <Link href="/products" className="btn btn-primary">
            Ürünleri Keşfet
          </Link>
          <Link href="/contact" className="btn btn-secondary">
            Destek Al
          </Link>
        </div>
      </section>

      <section>
        <h2>Öne Çıkan Ürünler</h2>
        <div className="grid">
          {featuredProducts.map((product) => (
            <article key={product.id} className="card">
              <span className="badge">{product.badge}</span>
              <h3>{product.name}</h3>
              <p className="price">{product.price}</p>
              <button className="btn btn-primary">Sepete Ekle</button>
            </article>
          ))}
        </div>
      </section>
    </div>
  );
}
