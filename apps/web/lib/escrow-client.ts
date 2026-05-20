export type EscrowCreateInput = {
  buyerEmail: string;
  finalCustomerPrice: number;
  itemTitle: string;
  itemDescription: string;
  currency?: string;
  sellerEmail?: string;
  feePayer?: "buyer" | "seller" | "split";
};

function requireEnv(name: string): string {
  const value = process.env[name];
  if (!value) throw new Error(`Missing required environment variable: ${name}`);
  return value;
}

export function getEscrowBaseUrl() {
  return requireEnv("ESCROW_API_BASE_URL");
}

export function getEscrowAuthHeader() {
  const email = requireEnv("ESCROW_EMAIL");
  const apiKey = requireEnv("ESCROW_API_KEY");
  const encoded = Buffer.from(`${email}:${apiKey}`).toString("base64");
  return `Basic ${encoded}`;
}

export async function createEscrowTransaction(input: EscrowCreateInput) {
  const baseUrl = getEscrowBaseUrl();
  const sellerEmail = input.sellerEmail ?? process.env.ESCROW_DEFAULT_SELLER_EMAIL ?? "me";
  const currency = input.currency ?? process.env.ESCROW_DEFAULT_CURRENCY ?? "usd";

  const payload = {
    parties: {
      buyer: { customer: input.buyerEmail, role: "buyer" },
      seller: { customer: sellerEmail, role: "seller" }
    },
    currency,
    description: "TradePi Globall Machinery Quote - Fine Cleaner 5X-5",
    items: [
      {
        title: input.itemTitle,
        description: input.itemDescription,
        type: "general_merchandise",
        category: "heavy_equipment_and_machinery",
        inspection_period: 259200,
        quantity: 1,
        schedule: [
          {
            amount: input.finalCustomerPrice,
            payer_customer: input.buyerEmail,
            beneficiary_customer: sellerEmail
          }
        ]
      }
    ],
    fee_split: input.feePayer ?? "buyer"
  };

  const res = await fetch(`${baseUrl}/transaction`, {
    method: "POST",
    headers: {
      Authorization: getEscrowAuthHeader(),
      "Content-Type": "application/json"
    },
    body: JSON.stringify(payload)
  });

  const data = await res.json();
  if (!res.ok) throw new Error(`Escrow create transaction failed: ${res.status}`);

  return { payload, data };
}

export async function fetchEscrowTransaction(transactionId: string) {
  const baseUrl = getEscrowBaseUrl();
  const res = await fetch(`${baseUrl}/transaction/${transactionId}`, {
    headers: { Authorization: getEscrowAuthHeader() }
  });
  const data = await res.json();
  if (!res.ok) throw new Error(`Escrow fetch transaction failed: ${res.status}`);
  return data;
}
