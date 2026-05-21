import Link from "next/link";
import { getAllMachineryProducts } from "@/lib/machinery-catalog";

export default function ProductsPage() {
  const products = getAllMachineryProducts();

  return (
    <div className="container page-stack">
      <section className="card">
        <p className="eyebrow">Ürünler</p>
        <h1>Machinery Catalog</h1>
        <p>
          Real supplier-catalog machinery candidates are listed below for quote-based RFQ workflow.
          Final configuration and commercial terms require supplier confirmation.
        </p>
      </section>

      <section className="grid product-grid">
        {products.map((product) => (
          <article className="card product-card" key={product.slug}>
            <div className="card" style={{ marginBottom: "1rem", background: "#f8fafc" }}>
              <p style={{ margin: 0, color: "#64748b" }}>Catalog image pending extraction</p>
            </div>
            <span className="badge">Quote-based</span>
            <p className="eyebrow">{product.category}</p>
            <h2>{product.name}</h2>
            <p>{product.short_description}</p>
            <div className="hero-actions">
              <Link className="btn btn-secondary" href={`/products/${product.slug}`}>
                Ürünü İncele
              </Link>
              <Link className="btn btn-primary" href="/request-quote">
                Teklif Al
              </Link>
            </div>
          </article>
        ))}
      </section>
    </div>
  );
}
