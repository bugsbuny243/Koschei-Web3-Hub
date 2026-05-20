import Link from "next/link";

const featuredProducts = [
  {
    id: 1,
    name: "5X-5 Fine Cleaner (Amiral Gemisi Komple Sistem)",
    partCode: "Fine Cleaner Model 5X-5",
    badge: "RFQ Active",
  },
  {
    id: 2,
    name: "LCSX Intelligent Photoelectric Color Sorter",
    partCode: "LCSX Cloud-Connected Sorting Series",
    badge: "RFQ Active",
  },
  {
    id: 3,
    name: "TQSF Gravity De-Stoner",
    partCode: "High-Capacity TQSF Series",
    badge: "RFQ Active",
  },
  {
    id: 4,
    name: "DCS Electronic Quantitative Packing Scale",
    partCode: "Automated DCS Filling & Stitching Station",
    badge: "RFQ Active",
  },
];

export default function HomePage() {
  return (
    <div className="page-stack">
      <section className="hero">
        <p className="eyebrow">Industrial Machinery Catalog</p>
        <h1>Official Product Data with Active RFQ Flow</h1>
        <p>
          Access exact model names, technical specifications, and part codes from official catalog
          sources. Request For Quote (RFO) submissions are active without price or payment terms.
        </p>
        <div className="hero-actions">
          <Link href="/products" className="btn btn-primary">
            View Product Data
          </Link>
          <Link href="/contact" className="btn btn-secondary">
            Submit RFQ Request
          </Link>
        </div>
      </section>

      <section>
        <h2>Catalog Models</h2>
        <div className="grid">
          {featuredProducts.map((product) => (
            <article key={product.id} className="card">
              <span className="badge">{product.badge}</span>
              <h3>{product.name}</h3>
              <p>
                <strong>Code:</strong> {product.partCode}
              </p>
              <Link href="/products" className="btn btn-primary">
                Request For Quote (RFO)
              </Link>
            </article>
          ))}
        </div>
      </section>
    </div>
  );
}
