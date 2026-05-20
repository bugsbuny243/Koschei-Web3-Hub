import fs from "node:fs/promises";
import path from "node:path";

type SupplierTerm = {
  supplier: string;
  product_slug: string;
  payment_terms: string;
  deposit_percent: number;
  balance_percent: number;
  voltage: string;
  freight_note: string;
  validity_note: string;
  private_notes: string;
};

async function getTerms(): Promise<SupplierTerm[]> {
  const filePath = path.join(process.cwd(), "data/internal/supplier-terms.json");
  return JSON.parse(await fs.readFile(filePath, "utf8")) as SupplierTerm[];
}

export default async function SupplierTermsPage({ searchParams }: { searchParams: Promise<{ password?: string }> }) {
  const { password } = await searchParams;
  if (!process.env.ADMIN_PASSWORD || password !== process.env.ADMIN_PASSWORD) {
    return <div className="page-stack"><h1>Admin Access Required</h1></div>;
  }
  const terms = await getTerms();
  return <div className="page-stack"><h1>Supplier Terms</h1><p>Internal supplier data. Do not publish prices, bank details or private negotiation notes.</p>{terms.map((t) => <article key={t.product_slug} className="card"><p><strong>supplier:</strong> {t.supplier}</p><p><strong>product slug:</strong> {t.product_slug}</p><p><strong>payment terms:</strong> {t.payment_terms}</p><p><strong>deposit percent:</strong> {t.deposit_percent}</p><p><strong>balance percent:</strong> {t.balance_percent}</p><p><strong>voltage:</strong> {t.voltage}</p><p><strong>freight note:</strong> {t.freight_note}</p><p><strong>validity note:</strong> {t.validity_note}</p><p><strong>private notes:</strong> {t.private_notes}</p></article>)}</div>;
}
