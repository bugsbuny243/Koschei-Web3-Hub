import { PRICING_PACKAGES } from "@/lib/pricing";

export default function PricingPage() {
  const { starter, pro } = PRICING_PACKAGES;

  return (
    <div className="container page-stack">
      <section className="card">
        <h1>Pricing</h1>
        <p>Choose flexible credit packs for Koschei tools.</p>
      </section>

      <section className="card product-card">
        <span className="badge">{starter.badge}</span>
        <h2>{starter.title}</h2>
        <p>
          <strong>{starter.priceLabel}</strong> • One-time digital credit package
        </p>
        <a
          className="btn btn-primary"
          href={starter.shopierUrl}
          target="_blank"
          rel="noopener noreferrer"
        >
          {starter.ctaLabel}
        </a>
      </section>

      <section className="card product-card">
        <span className="badge">{pro.badge}</span>
        <h2>{pro.title}</h2>
        <p>
          <strong>{pro.priceLabel}</strong>
        </p>
        <a
          className="btn btn-primary"
          href={pro.shopierUrl}
          target="_blank"
          rel="noopener noreferrer"
        >
          {pro.ctaLabel}
        </a>
      </section>
    </div>
  );
}
