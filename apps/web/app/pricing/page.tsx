const SHOPIER_STARTER_PACK_URL = "https://www.shopier.com/TradeVisual/47465449";

export default function PricingPage() {
  return (
    <div className="container page-stack">
      <section className="card">
        <h1>Pricing</h1>
        <p>Choose flexible credit packs for Koschei tools.</p>
      </section>

      <section className="card product-card">
        <span className="badge">Starter Pack</span>
        <h2>Koschei Starter Pack – 20.000 Credits</h2>
        <p><strong>899 TL</strong> • One-time digital credit package</p>
        <a
          className="btn btn-primary"
          href={SHOPIER_STARTER_PACK_URL}
          target="_blank"
          rel="noopener noreferrer"
        >
          Buy 20.000 Credits
        </a>
      </section>
    </div>
  );
}
