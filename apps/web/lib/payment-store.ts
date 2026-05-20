export type CustomerQuote = {
  id: string;
  quoteRequestId: string;
  customerName: string;
  buyerEmail: string;
  itemTitle: string;
  itemDescription: string;
  finalCustomerPrice: number;
  supplierLandedCost: number;
  status: "approved_internal" | "accepted_by_customer" | "draft";
};

export const customerQuotes: CustomerQuote[] = [
  {
    id: "cq-fine-cleaner-5x5-001",
    quoteRequestId: "qr-fine-cleaner-5x5-001",
    customerName: "Sample Buyer",
    buyerEmail: "buyer@example.com",
    itemTitle: "Fine Cleaner 5X-5 quote",
    itemDescription: "Fine Cleaner 5X-5 with control cabinet, fan/cyclone and selected screens.",
    finalCustomerPrice: 28500,
    supplierLandedCost: 23000,
    status: "accepted_by_customer"
  }
];

export const escrowTransactions: any[] = [];
export const supplierPayments: any[] = [];
export const webhookEvents: any[] = [];
