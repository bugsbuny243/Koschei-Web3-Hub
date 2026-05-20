export default function AboutPage() {
  return (
    <div className="container page-stack">
      <section className="card">
        <p className="eyebrow">Hakkımızda</p>
        <h1>TradePi Globall Machinery</h1>
        <p>
          TradePi Globall Machinery is building a quote-based B2B sourcing and quotation workflow
          for agricultural processing machinery. The platform collects buyer requirements,
          coordinates supplier-side cost/configuration review and prepares official customer
          quotations.
        </p>
      </section>

      <section className="card">
        <h2>Çalışma Yaklaşımı</h2>
        <ul className="check-list">
          <li>RFQ-first workflow</li>
          <li>No public fixed pricing</li>
          <li>Supplier-backed configuration review</li>
          <li>Logistics/customs/delivery cost evaluation</li>
          <li>Escrow-ready secure payment workflow after quote approval</li>
        </ul>
      </section>
    </div>
  );
}
