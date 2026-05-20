import fs from "node:fs/promises";
import path from "node:path";

type CatalogCandidate = {
  name: string;
  slug: string;
  source_page_image: string;
  status: string;
};

async function getCandidates(): Promise<CatalogCandidate[]> {
  const filePath = path.join(process.cwd(), "data/machinery/catalog-candidates.json");
  const fileContents = await fs.readFile(filePath, "utf8");
  return JSON.parse(fileContents) as CatalogCandidate[];
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

  const candidates = await getCandidates();

  return (
    <div className="page-stack">
      <section>
        <p className="eyebrow">Admin Only</p>
        <h1>Catalog Candidates</h1>
        <p>Draft catalogue candidate. Not public.</p>
      </section>

      <section className="grid">
        {candidates.map((candidate) => (
          <article key={candidate.slug} className="card">
            <h3>{candidate.name}</h3>
            <p>
              <strong>Slug:</strong> {candidate.slug}
            </p>
            <p>
              <strong>Source Page Image:</strong> {candidate.source_page_image}
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
