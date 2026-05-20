import Link from "next/link";

export default function ContactPage() {
  return (
    <div className="container page-stack">
      <section className="card">
        <p className="eyebrow">İletişim</p>
        <h1>Machinery RFQ Communication</h1>
        <p>For machinery requests, use the RFQ form.</p>
        <div className="hero-actions">
          <Link href="/request-quote" className="btn btn-primary">
            Teklif Formu Aç
          </Link>
        </div>
        <p className="muted-note">
          Direct contact details are confirmed during the official quotation process.
        </p>
      </section>
    </div>
  );
}
