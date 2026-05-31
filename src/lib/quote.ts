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

export function stripExportDisclaimers(exportNotes: string) {
  return exportNotes
    .replaceAll(HS_GTIP_DISCLAIMER, "")
    .replaceAll(PLATFORM_DISCLAIMER, "")
    .replace(/\n{3,}/g, "\n\n")
    .trim();
}

export function generateEnglishOfferText(company: CompanyInfo, buyer: BuyerInfo, product: ProductInfo) {
  const total = calculateTotal(product);
  const money = new Intl.NumberFormat("en-US", { style: "currency", currency: product.currency });

  return `Dear ${buyer.contactName},

Thank you for your interest in ${product.name}. We are pleased to submit our quotation for your review.

Product: ${product.name}
Quantity: ${product.quantity} ${product.unit}
Unit price: ${money.format(product.unitPrice)}
Total amount: ${money.format(total)}
Incoterm: ${product.incoterm}
Delivery time: ${product.deliveryTime}
Payment terms: ${product.paymentTerms}
Offer validity: ${product.validityDays} days

Please review the quotation and let us know if you would like to proceed or request any changes. We will be happy to support the next steps.

Kind regards,
${company.contactPerson}
${company.name}`;
}

export function generateFollowUpMessage(buyer: BuyerInfo, product: ProductInfo) {
  return `Hello ${buyer.contactName}, I’m sharing our quotation for ${product.quantity} ${product.unit} of ${product.name}. Please review the offer and let us know if you would like to proceed or request any changes. We’ll be happy to support the next steps.`;
}

export function generateProductDescriptionEn(product: ProductInfo) {
  return `${product.name} — ${product.category}. Seller-provided product description: ${product.descriptionTr}`;
}

export function generateExportNotes(product: ProductInfo) {
  return [
    product.hsCode ? `Estimated HS/GTIP code supplied for review: ${product.hsCode}.` : "No estimated HS/GTIP code was supplied.",
    product.packagingDetails ? `Packaging details: ${product.packagingDetails}` : "Packaging details were not supplied.",
    product.notes ? `Additional notes: ${product.notes}` : "",
  ].filter(Boolean).join("\n\n");
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
    exportNotes: stripExportDisclaimers(quote.exportNotes || fallback.exportNotes),
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
