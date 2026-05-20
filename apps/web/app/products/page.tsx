const products = [
  {
    name: "Fine Cleaner 5X-5",
    slug: "fine-cleaner-5x-5",
    description:
      "Current verified public product listing. Supply is managed via quote-based B2B workflow and final terms are confirmed per official quotation / proforma invoice.",
  },
];

export default function ProductsPage() {
  return (
    <div className="page-stack">
      <section>
        <p className="eyebrow">Industrial Product Data</p>
        <h1>Machine Catalog (Quote-based)</h1>
        <p>Final price confirmed per official proforma invoice.</p>
      </section>

      <section className="grid">
        {products.map((product) => (
          <article key={product.name} className="card">
            <h3>{product.name}</h3>
            <p>{product.description}</p>
            <p>
              <strong>Pricing:</strong> Quote-based
            </p>
            <div className="mt-4 flex gap-3">
              <a href={`/products/${product.slug}`} className="btn btn-primary">Request Quote</a>
            </div>
          </article>
        ))}
      </section>
    </div>
  );
}
