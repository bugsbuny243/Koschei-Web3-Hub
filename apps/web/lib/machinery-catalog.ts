import catalogCandidates from "../../../data/machinery/catalog-candidates.json";

export type MachineryProduct = {
  name: string;
  slug: string;
  category: string;
  short_description: string;
  source_pdf_page?: string;
  status: "supplier_catalog_verified";
  quote_based: true;
  public_pricing: false;
};

const products = catalogCandidates as MachineryProduct[];

export function getAllMachineryProducts(): MachineryProduct[] {
  return products;
}

export function getMachineryProductBySlug(slug: string): MachineryProduct | undefined {
  return products.find((product) => product.slug === slug);
}

export function getFeaturedMachineryProduct(): MachineryProduct | undefined {
  return products.find((product) => product.slug === "fine-cleaner-5x-5") ?? products[0];
}
