import Link from "next/link";
import { notFound } from "next/navigation";
import { getMachineryProductBySlug } from "@/lib/machinery-catalog";
import { machineryVideos } from "@/lib/machinery-media";
import { MachineryImage } from "./machinery-image";

export function MachineryProductDetail({
  slug,
  videoSectionTitle = "Product Videos",
}: {
  slug: string;
  videoSectionTitle?: string;
}) {
  const product = getMachineryProductBySlug(slug);

  if (!product) {
    notFound();
  }

  return (
    <div className="container page-stack">
      <section className="card">
        <MachineryImage
          imagePath={product.image_path}
          productName={product.name}
          width={1600}
          height={2200}
          style={{ width: "100%", height: "auto", marginBottom: "1rem", borderRadius: "0.5rem" }}
        />
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

      <section className="card">
        <h2>{videoSectionTitle}</h2>
        <div className="video-grid">
          {machineryVideos.map((video) => (
            <article className="video-card" key={video.id}>
              <div className="video-frame">
                <iframe
                  src={video.embedUrl}
                  title={video.title}
                  loading="lazy"
                  allow="accelerometer; autoplay; clipboard-write; encrypted-media; gyroscope; picture-in-picture; web-share"
                  allowFullScreen
                />
              </div>
              <p>{video.title}</p>
            </article>
          ))}
        </div>
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
