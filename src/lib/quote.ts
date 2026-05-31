import type { BuyerInfo, CompanyInfo, GeneratedQuoteContent, ProductInfo, QuoteData } from "@/lib/types";

const STORAGE_KEY = "teklifpilot.latestQuote";
const HISTORY_KEY = "teklifpilot.quotes";

export const HS_GTIP_DISCLAIMER = "HS/GTIP code is only an estimate and must be verified by a licensed customs broker or relevant authority before shipment.";
export const PLATFORM_DISCLAIMER = "TeklifPilot is an assistant for preparing commercial documents. It is not a customs broker, freight forwarder, escrow provider, legal advisor, or payment intermediary.";

export function generateQuotationNumber() {
  const now = new Date();
  const date = [now.getFullYear(), String(now.getMonth() + 1).padStart(2, "0"), String(now.getDate()).padStart(2, "0")].join("");
  const random = Math.random().toString(36).slice(2, 6).toUpperCase().padEnd(4, "0");
  return `TP-${date}-${random}`;
}

export function calculateTotal(product: ProductInfo) {
  return product.quantity * product.unitPrice;
}

export function generateEnglishOfferText(company: CompanyInfo, buyer: BuyerInfo, product: ProductInfo) {
  return `Dear ${buyer.contactName},\n\nThank you for your interest in our ${product.name}. On behalf of ${company.name}, we are pleased to submit our commercial quotation for ${product.quantity} ${product.unit} under ${product.incoterm} delivery terms.\n\nPlease find the product details, pricing, delivery timeline, and payment terms in this offer. Should you need any clarification or wish to discuss adjustments, we would be pleased to assist you.\n\nWe look forward to the opportunity to work with ${buyer.company} and to building a successful long-term business relationship.\n\nKind regards,\n${company.contactPerson}\n${company.name}`;
}

export function generateFollowUpMessage(buyer: BuyerInfo, product: ProductInfo) {
  return `Hello ${buyer.contactName}, I have shared our quotation for ${product.name}. The offer is valid for ${product.validityDays} days. Please let me know if you have any questions or if you would like to discuss the next steps. I would be happy to assist. Best regards.`;
}

export function generateProductDescriptionEn(product: ProductInfo) {
  return `${product.name} — ${product.category}. Seller-provided product description: ${product.descriptionTr}`;
}

export function generateExportNotes(product: ProductInfo) {
  const details = [
    product.hsCode ? `Estimated HS/GTIP code supplied for review: ${product.hsCode}.` : "No estimated HS/GTIP code was supplied.",
    product.packagingDetails ? `Packaging details: ${product.packagingDetails}` : "Packaging details were not supplied.",
    product.notes ? `Additional notes: ${product.notes}` : "",
  ].filter(Boolean);

  return [...details, HS_GTIP_DISCLAIMER, PLATFORM_DISCLAIMER].join("\n\n");
}

export function createFallbackQuoteContent(company: CompanyInfo, buyer: BuyerInfo, product: ProductInfo): GeneratedQuoteContent {
  return {
    englishOfferText: generateEnglishOfferText(company, buyer, product),
    followUpMessage: generateFollowUpMessage(buyer, product),
    productDescriptionEn: generateProductDescriptionEn(product),
    exportNotes: generateExportNotes(product),
    usedFallback: true,
  };
}

function normalizeStoredQuote(quote: QuoteData): QuoteData {
  const fallback = createFallbackQuoteContent(quote.company, quote.buyer, quote.product);
  return {
    ...quote,
    productDescriptionEn: quote.productDescriptionEn || fallback.productDescriptionEn,
    exportNotes: quote.exportNotes || fallback.exportNotes,
    usedFallback: quote.usedFallback ?? true,
  };
}

export function saveQuoteToLocalStorage(quote: QuoteData) {
  if (typeof window === "undefined") return;
  localStorage.setItem(STORAGE_KEY, JSON.stringify(quote));
  const history = getQuoteHistory();
  localStorage.setItem(HISTORY_KEY, JSON.stringify([quote, ...history].slice(0, 25)));
}

export function getLatestQuoteFromLocalStorage(): QuoteData | null {
  if (typeof window === "undefined") return null;
  const stored = localStorage.getItem(STORAGE_KEY);
  if (!stored) return null;
  try { return normalizeStoredQuote(JSON.parse(stored) as QuoteData); } catch { return null; }
}

export function getQuoteHistory(): QuoteData[] {
  if (typeof window === "undefined") return [];
  const stored = localStorage.getItem(HISTORY_KEY);
  if (!stored) return [];
  try { return (JSON.parse(stored) as QuoteData[]).map(normalizeStoredQuote); } catch { return []; }
}
