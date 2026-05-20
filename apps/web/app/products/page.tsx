const products = [
  {
    name: "Fine Cleaner 5X-5",
    partCode: "Fine Cleaner Model 5X-5",
    description:
      "Capacity: 5 TPH (Tons Per Hour) based on wheat processing density. Total Power Requirement: Combined multi-motor load (6.7KW + 7.5KW + 1.1KW + 1.1KW) running on 380V 50Hz 3-Phase power grids. Physical Dimensions: Main body size 3200 × 1940 × 3600 mm, total net system mass 3250kg + 4000kg auxiliary weights. Interchangeable Sifters: Delivered with 1 suit of 7PCS custom spare sifters engineered specifically for white bean calibration.",
  },
];

export default function ProductsPage() {
  return (
    <div className="page-stack">
      <section>
        <p className="eyebrow">Industrial Product Data</p>
        <h1>Machine Catalog (RFQ Active)</h1>
        <p>
          Public product listing currently includes only Fine Cleaner 5X-5. Supplier catalogue source-page
          evidence remains restricted to admin candidate review and is never published as product images.
        </p>
      </section>
      <section className="grid">
        {products.map((product) => (
          <article key={product.name} className="card">
            <h3>{product.name}</h3>
            <p>
              <strong>Exact Part & Model Code:</strong> {product.partCode}
            </p>
            <p>{product.description}</p>
            <a className="btn btn-primary" href="/request-quote">Request Quote</a>
          </article>
        ))}
      </section>
    </div>
  );
}
