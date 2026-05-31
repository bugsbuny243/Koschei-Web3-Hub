export type Currency = "USD" | "EUR" | "TRY";
export type Incoterm = "EXW" | "FOB" | "CIF" | "DAP";

export interface CompanyInfo {
  name: string;
  contactPerson: string;
  email: string;
  phone: string;
  address: string;
  website: string;
  logoUrl?: string;
}

export interface BuyerInfo {
  company: string;
  contactName: string;
  country: string;
  email: string;
}

export interface ProductInfo {
  name: string;
  category: string;
  descriptionTr: string;
  quantity: number;
  unit: string;
  unitPrice: number;
  currency: Currency;
  incoterm: Incoterm;
  deliveryTime: string;
  paymentTerms: string;
  validityDays: number;
  hsCode?: string;
  packagingDetails: string;
  notes: string;
}

export interface QuoteData {
  quotationNumber: string;
  createdAt: string;
  company: CompanyInfo;
  buyer: BuyerInfo;
  product: ProductInfo;
  englishOfferText: string;
  followUpMessage: string;
}
