import Link from "next/link";
import { notFound } from "next/navigation";
import { getMachineryProductBySlug } from "@/lib/machinery-catalog";

export function MachineryProductDetail({ slug }: { slug: string }) {
  const product = getMachineryProductBySlug(slug);

  if (!product) {
    notFound();
  }

  return (
    <div className="container page-stack">
      <section className="card">
        <p className="eyebrow">Product Detail</p>
        <h1>{product.name}</h1>
        <p>{product.short_description}</p>
      </section>
      <section className="card two-col">
        <div>
          <h2>Category</h2>
          <p>{product.category}</p>
        </div>
        <div>
          <h2>Commercial Model</h2>
          <span className="badge">Quote-based</span>
        </div>
      </section>
      <section className="card">
        <h2>No Public Price Notice</h2>
        <p>
          Public pricing is not shown. Final configuration and final price are provided only after
          supplier confirmation and RFQ review.
        </p>
      </section>
      <section className="card">
        <h2>Supplier Confirmation Required</h2>
        <p>
          Capacity, scope of supply, electrical standard, optional accessories, delivery and trade
          terms must be confirmed by the supplier before quotation finalization.
        </p>
        <Link href="/request-quote" className="btn btn-primary">
          Teklif Al (RFQ)
        </Link>
      </section>
      {product.source_pdf_page ? (
        <section className="card">
          <h2>Source Catalog Reference</h2>
          <p>{product.source_pdf_page}</p>
        </section>
      ) : null}
    </div>
  );
}
