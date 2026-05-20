const products = [
  { name: "Katlanabilir Laptop Standı", price: "₺729", description: "Ergonomik çalışma için ayarlanabilir açı." },
  { name: "Manyetik Araç Telefon Tutucu", price: "₺349", description: "Güçlü mıknatısla güvenli sürüş deneyimi." },
  { name: "Akıllı Nem Ölçer", price: "₺569", description: "Ev bitkilerini sağlıklı tutmak için ideal." },
  { name: "Taşınabilir Mini Yazıcı", price: "₺1.199", description: "Telefonundan anında baskı al." },
];

export default function ProductsPage() {
  return (
    <div className="page-stack">
      <section>
        <p className="eyebrow">Katalog</p>
        <h1>Trend Ürünler</h1>
        <p>Her hafta güncellenen viral ürünleri buradan inceleyebilirsin.</p>
      </section>
      <section className="grid">
        {products.map((product) => (
          <article key={product.name} className="card">
            <h3>{product.name}</h3>
            <p>{product.description}</p>
            <p className="price">{product.price}</p>
            <button className="btn btn-primary">Şimdi Satın Al</button>
          </article>
        ))}
      </section>
    </div>
  );
}
