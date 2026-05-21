import Link from "next/link";
import { getAllMachineryProducts, getFeaturedMachineryProduct } from "@/lib/machinery-catalog";
import { machineryVideos } from "@/lib/machinery-media";

const workflowSteps = [
  "Customer submits crop, location, capacity and delivery requirements.",
  "TradePi Globall validates RFQ details and detects missing importer/company information.",
  "TradePi Globall drafts supplier-ready English inquiry for Cathy and receives supplier DDP proforma terms.",
  "TradePi Globall records supplier-confirmed DDP quote and applies internal commission workflow.",
  "TradePi Globall prepares one final customer quotation after admin approval.",
  "Payment workflow can be arranged after quote approval.",
];

export default function HomePage() {
  const featured = getFeaturedMachineryProduct();
  const catalogPreview = getAllMachineryProducts().slice(0, 8);

  return (
    <div className="container page-stack">
      <section className="hero">
        <p className="eyebrow">TradePi Globall Machinery</p>
        <h1>Commission-Based RFQ Brokerage for Agricultural Machinery</h1>
        <p>
          TradePi Globall is a quote-based B2B RFQ and secure payment coordination platform for
          agricultural machinery. TradePi does not manufacture, ship, insure, clear customs, or
          guarantee supplier delivery.
        </p>
        <div className="hero-actions">
          <Link href="/request-quote" className="btn btn-primary">
            Teklif Al
          </Link>
          {featured ? (
            <Link href={`/products/${featured.slug}`} className="btn btn-secondary">
              {featured.name} İncele
            </Link>
          ) : null}
        </div>
      </section>

      <section className="card">
        <h2>Current Featured Product</h2>
        <h3>{featured?.name ?? "Fine Cleaner 5X-5"}</h3>
        <p>
          Fine Cleaner 5X-5 remains featured, while the broader supplier machinery catalog is now
          available for RFQ-based review.
        </p>
      </section>

      <section className="card">
        <h2>Machinery Catalog</h2>
        <p>
          Supplier catalog candidates are published without public pricing. Final machine scope and
          quotation require supplier confirmation.
        </p>
        <div className="grid product-grid">
          {catalogPreview.map((product) => (
            <article className="card product-card" key={product.slug}>
              <p className="eyebrow">{product.category}</p>
              <h3>{product.name}</h3>
              <p>{product.short_description}</p>
              <Link href={`/products/${product.slug}`} className="btn btn-secondary">
                Ürünü İncele
              </Link>
            </article>
          ))}
        </div>
        <div className="hero-actions" style={{ marginTop: "1rem" }}>
          <Link href="/products" className="btn btn-primary">
            Tüm Kataloğu Gör
          </Link>
        </div>
      </section>


      <section className="card">
        <h2>Machinery Videos</h2>
        <div className="video-grid">
          {machineryVideos.map((video) => (
            <article className="video-card" key={video.id}>
              <div className="video-frame">
                <iframe
                  src={video.embedUrl}
                  title={video.title}
                  loading="lazy"
                  allow="accelerometer; autoplay; clipboard-write; encrypted-media; gyroscope; picture-in-picture; web-share"
                  allowFullScreen
                />
              </div>
              <p>{video.title}</p>
            </article>
          ))}
        </div>
      </section>

      <section className="card">
        <h2>How the RFQ Workflow Works</h2>
        <ol className="step-list">
          {workflowSteps.map((step) => (
            <li key={step}>{step}</li>
          ))}
        </ol>
      </section>
    </div>
  );
}
