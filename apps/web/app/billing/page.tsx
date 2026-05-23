import { PRICING_PACKAGES } from "@/lib/pricing";

export default function BillingPage() {
  const { starter, pro } = PRICING_PACKAGES;

  return (
    <div className="container page-stack">
      <section className="card">
        <h1>Billing</h1>
        <p>Pay for credits, then submit your payment details for manual activation.</p>
      </section>

      <section className="card">
        <h2>Payment Options</h2>

        <div className="page-stack" style={{ marginTop: "0.5rem" }}>
          <div>
            <p>
              <strong>{starter.title}</strong>
            </p>
            <p>
              <strong>{starter.priceLabel}</strong>
            </p>
            <a
              className="btn btn-primary"
              href={starter.shopierUrl}
              target="_blank"
              rel="noopener noreferrer"
            >
              Buy on Shopier
            </a>
          </div>

          <div>
            <p>
              <strong>{pro.title}</strong>
            </p>
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
          </div>
        </div>

        <p className="muted-note" style={{ marginTop: "0.75rem" }}>
          After payment, return to this page and submit your Koschei account email and payment reference. Your credits will be activated manually by the owner.
        </p>
      </section>

      <section className="card">
        <h2>Payment Submission Form</h2>
        <form className="rfq-form">
          <div className="form-grid">
            <label>
              Email
              <input type="email" name="email" placeholder="you@example.com" required />
            </label>

            <label>
              Selected package
              <input type="text" name="selectedPackage" defaultValue={starter.title} readOnly />
            </label>

            <label>
              Payment provider
              <input type="text" name="paymentProvider" defaultValue="Shopier" readOnly />
            </label>

            <label>
              Payment reference / order number
              <input
                type="text"
                name="paymentReference"
                placeholder="Enter your Shopier order number"
                required
              />
            </label>

            <label className="full-width">
              Note
              <textarea
                name="note"
                rows={4}
                placeholder="Add any additional context for manual credit activation."
              />
            </label>
          </div>
        </form>
      </section>
    </div>
  );
}
