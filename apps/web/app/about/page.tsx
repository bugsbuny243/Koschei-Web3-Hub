export default function AboutPage() {
  return (
    <div className="container page-stack">
      <section className="card">
        <p className="eyebrow">Hakkımızda</p>
        <h1>TradePi Globall Machinery</h1>
        <p>
          TradePi Globall Machinery operates as a commission-based RFQ brokerage workflow for
          agricultural processing machinery. The platform collects buyer requirements, prepares
          supplier-ready RFQ messages, coordinates secure payment workflow, and prepares official
          customer-facing offers after supplier confirmation.
        </p>
      </section>

      <section className="card">
        <h2>Çalışma Yaklaşımı</h2>
        <ul className="check-list">
          <li>RFQ-first workflow</li>
          <li>No public fixed pricing</li>
          <li>Supplier-backed configuration and DDP confirmation</li>
          <li>Commission-based brokerage (not margin-based resale)</li>
          <li>Escrow-ready secure payment workflow after quote approval</li>
        </ul>
      </section>
    </div>
  );
}
