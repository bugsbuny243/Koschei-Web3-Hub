import "server-only";

type EscrowConfig = {
  env: string;
  baseUrl: string;
  email: string;
  apiKey: string;
  defaultSellerEmail: string;
  defaultCurrency: string;
  feePayer: string;
  webhookToken?: string;
};

function requiredEnv(name: keyof NodeJS.ProcessEnv): string {
  const value = process.env[name];
  if (!value) {
    throw new Error(`Missing required environment variable: ${name}`);
  }
  return value;
}

export function getEscrowConfig(): EscrowConfig {
  return {
    env: requiredEnv("ESCROW_ENV"),
    baseUrl: requiredEnv("ESCROW_API_BASE_URL"),
    email: requiredEnv("ESCROW_EMAIL"),
    apiKey: requiredEnv("ESCROW_API_KEY"),
    defaultSellerEmail: requiredEnv("ESCROW_DEFAULT_SELLER_EMAIL"),
    defaultCurrency: requiredEnv("ESCROW_DEFAULT_CURRENCY"),
    feePayer: requiredEnv("ESCROW_FEE_PAYER"),
    webhookToken: process.env.ESCROW_WEBHOOK_TOKEN
  };
}

export function getEscrowBaseUrl(): string {
  return getEscrowConfig().baseUrl;
}

export function getEscrowAuthHeader(): string {
  const { email, apiKey } = getEscrowConfig();
  const token = Buffer.from(`${email}:${apiKey}`).toString("base64");
  return `Basic ${token}`;
}

export async function escrowFetch(path: string, options: RequestInit = {}): Promise<Response> {
  const baseUrl = getEscrowBaseUrl();
  const normalizedPath = path.startsWith("/") ? path : `/${path}`;
  const url = `${baseUrl}${normalizedPath}`;

  const headers = new Headers(options.headers);
  headers.set("Authorization", getEscrowAuthHeader());
  headers.set("Accept", "application/json");
  if (!headers.has("Content-Type") && options.body) {
    headers.set("Content-Type", "application/json");
  }

  const response = await fetch(url, {
    ...options,
    headers,
    cache: "no-store"
  });

  if (!response.ok) {
    throw new Error(`Escrow API request failed (${response.status} ${response.statusText})`);
  }

  return response;
}

export async function fetchEscrowTransaction(transactionId: string): Promise<unknown> {
  if (!transactionId) {
    throw new Error("Transaction ID is required");
  }
  const response = await escrowFetch(`/transaction/${encodeURIComponent(transactionId)}`);
  return response.json();
}
