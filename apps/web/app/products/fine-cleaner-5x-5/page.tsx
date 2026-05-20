import Link from "next/link";

export default function FineCleanerPage() {
  return (
    <div className="container page-stack">
      <section className="card">
        <p className="eyebrow">Product Detail</p>
        <h1>Fine Cleaner 5X-5</h1>
        <p>
          Fine Cleaner 5X-5 is listed for professional RFQ-based procurement in agricultural
          processing projects.
        </p>
      </section>

      <section className="card">
        <h2>Product Overview</h2>
        <p>
          This machine is currently the verified public product for customer RFQ collection and
          commercial evaluation.
        </p>
      </section>

      <section className="card">
        <h2>Configuration Under Review</h2>
        <ul className="check-list">
          <li>Fine Cleaner 5X-5 main machine body</li>
          <li>Control cabinet</li>
          <li>Fan</li>
          <li>Cyclone dust collection system</li>
          <li>Low-speed / anti-broken bucket elevator</li>
          <li>Screen sets for wheat, barley and white bean subject to final confirmation</li>
          <li>380V 50Hz 3 phase compatibility</li>
        </ul>
      </section>

      <section className="card two-col">
        <div>
          <h2>Possible Crop Applications</h2>
          <p>
            Intended for agricultural grain and pulse cleaning projects where final crop alignment
            is confirmed during supplier-side review.
          </p>
        </div>
        <div>
          <h2>RFQ-Required Information</h2>
          <p>
            Submit crop type, required capacity, destination details, installation conditions and
            preferred trade terms to receive a finalized quotation.
          </p>
        </div>
      </section>

      <section className="card">
        <h2>No Public Pricing Notice</h2>
        <p>
          Price is not displayed publicly. Final amount is confirmed only after supplier
          configuration validation, logistics and commercial review, and official quotation/proforma
          preparation.
        </p>
        <Link href="/request-quote" className="btn btn-primary">
          Request Quote
        </Link>
      </section>
    </div>
  );
}
