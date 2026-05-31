import type { BuyerInfo, CompanyInfo, ProductInfo, QuoteData } from "@/lib/types";

const STORAGE_KEY = "teklifpilot.latestQuote";
const HISTORY_KEY = "teklifpilot.quotes";

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
  return `Dear ${buyer.contactName},\n\nThank you for your interest in our ${product.name}. On behalf of ${company.name}, we are pleased to submit our commercial quotation for your consideration. The offer has been prepared based on the requested quantity of ${product.quantity} ${product.unit} and the ${product.incoterm} delivery term.\n\nWe would be glad to answer your questions and discuss the next steps for a smooth export process. We look forward to establishing a successful business relationship with ${buyer.company}.\n\nKind regards,\n${company.contactPerson}\n${company.name}`;
}

export function generateFollowUpMessage(buyer: BuyerInfo, product: ProductInfo) {
  return `Hello ${buyer.contactName}, I have shared our quotation for ${product.name}. The offer is valid for ${product.validityDays} days. Please let me know if you have any questions or would like to discuss the next steps. Best regards.`;
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
  try { return JSON.parse(stored) as QuoteData; } catch { return null; }
}

export function getQuoteHistory(): QuoteData[] {
  if (typeof window === "undefined") return [];
  const stored = localStorage.getItem(HISTORY_KEY);
  if (!stored) return [];
  try { return JSON.parse(stored) as QuoteData[]; } catch { return []; }
}
