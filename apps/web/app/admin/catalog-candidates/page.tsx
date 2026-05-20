import fs from "node:fs/promises";
import path from "node:path";

type CatalogCandidate = {
  name: string;
  slug: string;
  source_page_image: string;
  status: string;
};

const SOURCE_PAGE_DIR = path.join(
  process.cwd(),
  "apps/web/public/machinery/source-pages/maosheng-catalog",
);

async function getCandidates(): Promise<CatalogCandidate[]> {
  const filePath = path.join(process.cwd(), "data/machinery/catalog-candidates.json");
  const fileContents = await fs.readFile(filePath, "utf8");
  return JSON.parse(fileContents) as CatalogCandidate[];
}

async function getUploadedSourcePages(): Promise<string[]> {
  const entries = await fs.readdir(SOURCE_PAGE_DIR, { withFileTypes: true });

  return entries
    .filter((entry) => entry.isFile())
    .map((entry) => entry.name)
    .filter((name) => /\.(jpg|jpeg|png|webp)$/i.test(name))
    .filter((name) => !name.toLowerCase().includes("com.android.chrome"))
    .sort((a, b) => a.localeCompare(b, undefined, { numeric: true }));
}

export default async function CatalogCandidatesPage({
  searchParams,
}: {
  searchParams: Promise<{ password?: string }>;
}) {
  const { password } = await searchParams;
  const adminPassword = process.env.ADMIN_PASSWORD;

  if (!adminPassword || password !== adminPassword) {
    return (
      <div className="page-stack">
        <section>
          <p className="eyebrow">Admin Access Required</p>
          <h1>Catalog Candidate Review</h1>
          <p>
            This page is protected. Add <code>?password=...</code> to the URL with a valid
            ADMIN_PASSWORD value.
          </p>
        </section>
      </div>
    );
  }

  const [candidates, uploadedSourcePages] = await Promise.all([
    getCandidates(),
    getUploadedSourcePages(),
  ]);

  return (
    <div className="page-stack">
      <section>
        <p className="eyebrow">Admin Only</p>
        <h1>Catalog Candidates</h1>
        <p>Draft catalogue candidate. Not public.</p>
        <p>
          <strong>
            These are source-page evidence screenshots. They are not public product images. Crop and
            approve product-specific images before publishing.
          </strong>
        </p>
      </section>

      <section>
        <h2>Uploaded source pages</h2>
        <p>
          Auto-detected from <code>/machinery/source-pages/maosheng-catalog/</code> (JPG/PNG/WEBP,
          excluding obvious GitHub UI screenshots).
        </p>
        <div className="grid">
          {uploadedSourcePages.map((fileName) => {
            const publicPath = `/machinery/source-pages/maosheng-catalog/${fileName}`;
            return (
              <article key={fileName} className="card">
                <img src={publicPath} alt={`Source page evidence ${fileName}`} loading="lazy" />
                <p>
                  <strong>File:</strong> {fileName}
                </p>
                <p>
                  <em>Source-page evidence only. Never publish directly as a product image.</em>
                </p>
              </article>
            );
          })}
        </div>
      </section>

      <section className="grid">
        {candidates.map((candidate) => (
          <article key={candidate.slug} className="card">
            <h3>{candidate.name}</h3>
            <p>
              <strong>Slug:</strong> {candidate.slug}
            </p>
            <p>
              <strong>Source Page Evidence:</strong> {candidate.source_page_image}
            </p>
            <p>
              <em>Evidence-only asset for admin review. Not a public product image.</em>
            </p>
            <p>
              <strong>Status:</strong> {candidate.status}
            </p>
            <p>
              <strong>Note:</strong> Draft catalogue candidate. Not public.
            </p>
          </article>
        ))}
      </section>
    </div>
  );
}
