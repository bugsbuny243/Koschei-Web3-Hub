import Link from "next/link";

export default function ProductsPage() {
  return (
    <div className="container page-stack">
      <section className="card">
        <p className="eyebrow">Ürünler</p>
        <h1>Current Verified Public Product</h1>
        <p>
          TradePi Globall publishes only verified machinery suitable for quote-based RFQ workflow.
        </p>
      </section>

      <section className="grid single-grid">
        <article className="card product-card">
          <span className="badge">Quote-based</span>
          <h2>Fine Cleaner 5X-5</h2>
          <p>
            Current verified public machinery listing for agricultural processing applications.
            Configuration details are finalized through supplier-backed RFQ review.
          </p>
          <div className="hero-actions">
            <Link className="btn btn-secondary" href="/products/fine-cleaner-5x-5">
              Ürünü İncele
            </Link>
            <Link className="btn btn-primary" href="/request-quote">
              Teklif Al
            </Link>
          </div>
        </article>
      </section>
    </div>
  );
}
