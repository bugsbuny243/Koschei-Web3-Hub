import Link from "next/link";

const workflowSteps = [
  "Customer submits crop, location, capacity and delivery requirements.",
  "TradePi Globall reviews the technical and commercial request.",
  "Supplier-side machine configuration and landed cost are calculated internally.",
  "TradePi Globall prepares one final customer quotation.",
  "Payment workflow can be arranged after quote approval.",
];

export default function HomePage() {
  return (
    <div className="container page-stack">
      <section className="hero">
        <p className="eyebrow">TradePi Globall Machinery</p>
        <h1>Agricultural Machinery Supply, Quotation &amp; Logistics Coordination</h1>
        <p>
          TradePi Globall coordinates quote-based B2B sourcing for agricultural processing
          machinery. Final configuration, freight, customs, taxes, delivery terms and payment
          terms are confirmed only through RFQ review and official proforma invoice.
        </p>
        <div className="hero-actions">
          <Link href="/request-quote" className="btn btn-primary">
            Teklif Al
          </Link>
          <Link href="/products/fine-cleaner-5x-5" className="btn btn-secondary">
            Fine Cleaner 5X-5 İncele
          </Link>
        </div>
        <ul className="trust-list">
          <li>Quote-based B2B workflow</li>
          <li>No public fixed pricing</li>
          <li>Supplier-backed configuration review</li>
          <li>Escrow-ready payment workflow after quote approval</li>
        </ul>
      </section>

      <section className="card">
        <h2>No Public Price Listing</h2>
        <p>
          Heavy machinery pricing depends on machine configuration, crop type, capacity
          requirement, spare screen sets, delivery location, trade term, freight, customs, taxes
          and shipment date. TradePi Globall does not display fixed public prices. Final price is
          confirmed by official quotation/proforma invoice only.
        </p>
      </section>

      <section className="card">
        <h2>Current Verified Public Product</h2>
        <h3>Fine Cleaner 5X-5</h3>
        <p>
          Fine Cleaner 5X-5 is the current verified product prepared for public RFQ listing.
          Configuration can include control cabinet, fan, cyclone dust collection, low-speed bucket
          elevator and crop-specific screen sets, subject to supplier confirmation.
        </p>
        <Link href="/request-quote" className="btn btn-primary">
          Request Quote
        </Link>
      </section>

      <section className="card">
        <h2>How the RFQ Workflow Works</h2>
        <ol className="step-list">
          {workflowSteps.map((step) => (
            <li key={step}>{step}</li>
          ))}
        </ol>
      </section>

      <section className="card">
        <h2>Transparent Scope, Private Costing</h2>
        <p>
          Supplier cost, logistics cost breakdown, TradePi margin, Escrow.com fees and internal
          operating costs are not displayed publicly. Customers receive only the final official
          quotation amount and terms.
        </p>
      </section>
    </div>
  );
}
