import "server-only";

export type EscrowTransactionResponse = {
  id?: number | string;
  status?: string;
  [key: string]: unknown;
};

type EscrowConfig = {
  env: string;
  baseUrl: string;
  email: string;
  apiKey: string;
  defaultSellerEmail?: string;
  defaultCurrency: string;
  feePayer: string;
  webhookToken?: string;
};

function requiredEnv(name: keyof NodeJS.ProcessEnv): string {
  const value = process.env[name];
  if (!value) throw new Error(`Missing required environment variable: ${name}`);
  return value;
}

export function getEscrowConfig(): EscrowConfig {
  return {
    env: requiredEnv("ESCROW_ENV"),
    baseUrl: requiredEnv("ESCROW_API_BASE_URL"),
    email: requiredEnv("ESCROW_EMAIL"),
    apiKey: requiredEnv("ESCROW_API_KEY"),
    defaultSellerEmail: process.env.ESCROW_DEFAULT_SELLER_EMAIL,
    defaultCurrency: process.env.ESCROW_DEFAULT_CURRENCY ?? "usd",
    feePayer: process.env.ESCROW_FEE_PAYER ?? "tradepi_globall",
    webhookToken: process.env.ESCROW_WEBHOOK_TOKEN
  };
}

function getEscrowAuthHeader(): string {
  const { email, apiKey } = getEscrowConfig();
  return `Basic ${Buffer.from(`${email}:${apiKey}`).toString("base64")}`;
}

export async function escrowFetch(path: string, options: RequestInit = {}): Promise<Response> {
  const url = `${getEscrowConfig().baseUrl}${path.startsWith("/") ? path : `/${path}`}`;
  const headers = new Headers(options.headers);
  headers.set("Authorization", getEscrowAuthHeader());
  headers.set("Accept", "application/json");
  if (!headers.has("Content-Type") && options.body) headers.set("Content-Type", "application/json");
  const res = await fetch(url, { ...options, headers, cache: "no-store" });
  if (!res.ok) throw new Error(`Escrow API request failed (${res.status})`);
  return res;
}

export async function createEscrowTransaction(input: {
  buyerEmail: string;
  finalCustomerPrice: number;
  itemTitle: string;
  itemDescription: string;
}) {
  const cfg = getEscrowConfig();
  const seller = cfg.defaultSellerEmail || "me";
  const payload = {
    parties: [
      { role: "buyer", customer: input.buyerEmail },
      { role: "seller", customer: seller }
    ],
    currency: cfg.defaultCurrency || "usd",
    description: "TradePi Globall Machinery Quote - Fine Cleaner 5X-5",
    items: [
      {
        title: input.itemTitle,
        description: input.itemDescription,
        type: "general_merchandise",
        inspection_period: 259200,
        quantity: 1,
        schedule: [{ amount: input.finalCustomerPrice, payer_customer: input.buyerEmail, beneficiary_customer: seller }]
      }
    ]
  };
  const response = await escrowFetch("/transaction", { method: "POST", body: JSON.stringify(payload) });
  const data = (await response.json()) as EscrowTransactionResponse;
  return { payload, data };
}

export async function fetchEscrowTransaction(transactionId: string): Promise<EscrowTransactionResponse> {
  const response = await escrowFetch(`/transaction/${encodeURIComponent(transactionId)}`);
  return (await response.json()) as EscrowTransactionResponse;
}
