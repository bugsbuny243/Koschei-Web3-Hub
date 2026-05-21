import Link from "next/link";
import { getAllMachineryProducts } from "@/lib/machinery-catalog";
import { machineryVideos } from "@/lib/machinery-media";
import { MachineryImage } from "./machinery-image";

export default function ProductsPage() {
  const products = getAllMachineryProducts();

  return (
    <div className="container page-stack">
      <section className="card">
        <p className="eyebrow">Ürünler</p>
        <h1>Machinery Catalog</h1>
        <p>
          Real supplier-catalog machinery candidates are listed below for quote-based RFQ workflow.
          Final configuration and commercial terms require supplier confirmation.
        </p>
      </section>


      <section className="card">
        <h2>Machinery Videos</h2>
        <p>Preview real TradePi Globall machinery videos before reviewing full catalog entries.</p>
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

      <section className="grid product-grid">
        {products.map((product) => (
          <article className="card product-card" key={product.slug}>
            <div className="card" style={{ marginBottom: "1rem", background: "#f8fafc" }}>
              <MachineryImage
                imagePath={product.image_path}
                imageStatus={product.image_status}
                productName={product.name}
                width={1200}
                height={1600}
                style={{ width: "100%", height: "auto", borderRadius: "0.5rem" }}
              />
            </div>
            <span className="badge">Quote-based</span>
            <p className="eyebrow">{product.category}</p>
            <h2>{product.name}</h2>
            <p>{product.short_description}</p>
            <div className="hero-actions">
              <Link className="btn btn-secondary" href={`/products/${product.slug}`}>
                Ürünü İncele
              </Link>
              <Link className="btn btn-primary" href="/request-quote">
                Teklif Al
              </Link>
            </div>
          </article>
        ))}
      </section>
    </div>
  );
}
