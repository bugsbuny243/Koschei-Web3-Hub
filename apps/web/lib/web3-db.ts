import "server-only";
import { Pool } from "pg";
import { web3Env } from "@/lib/web3-env";

export type ChainRow = { id: string; slug: string; name: string; is_active: boolean };
export type InvoiceRow = {
  id: string;
  chain_slug: string;
  stablecoin_symbol: string;
  stablecoin_contract: string;
  receiver_address: string;
  expected_amount: string;
  currency: string;
  due_at: string | null;
  paid_at: string | null;
  status: string;
  metadata: Record<string, unknown> | null;
  created_at: string;
};
export type PaymentEventRow = {
  id: string;
  invoice_id: string | null;
  chain_slug: string;
  tx_hash: string;
  log_index: number | null;
  from_address: string;
  to_address: string;
  token_contract: string;
  token_symbol: string;
  token_decimals: number;
  amount: string;
  raw_amount: string | null;
  block_number: number | null;
  status: string;
  raw_event: Record<string, unknown> | null;
  created_at: string;
};

let pool: Pool | null = null;

function getPool() {
  const connectionString = web3Env.DATABASE_URL;
  if (!connectionString) {
    throw new Error("DATABASE_URL is required for database access");
  }

  if (!pool) {
    pool = new Pool({
      connectionString,
      connectionTimeoutMillis: 5000,
      idleTimeoutMillis: 5000,
      allowExitOnIdle: true
    });
  }

  return pool;
}

export const web3Db = {
  query: <T>(text: string, params?: unknown[]) => getPool().query<T>(text, params),

  chains: {
    async listActive() {
      const { rows } = await getPool().query<ChainRow>(
        `select id::text, slug, name, is_active from web3_chains where is_active = true order by name asc`
      );
      return rows;
    },
    async bySlug(slug: string) {
      const { rows } = await getPool().query<ChainRow>(
        `select id::text, slug, name, is_active from web3_chains where slug = $1 limit 1`,
        [slug]
      );
      return rows[0] ?? null;
    }
  },
  invoices: {
    async list() {
      const { rows } = await getPool().query<InvoiceRow>(
        `select i.id::text, i.chain_slug, i.stablecoin_symbol, i.stablecoin_contract, i.receiver_address,
                i.expected_amount::text, i.currency, i.due_at::text, i.paid_at::text, i.status, i.metadata, i.created_at::text
         from web3_invoices i
         order by i.created_at desc`
      );
      return rows;
    },
    async create(input: Omit<InvoiceRow, "id" | "paid_at" | "status" | "created_at">) {
      const { rows } = await getPool().query<InvoiceRow>(
        `insert into web3_invoices (chain_slug, stablecoin_symbol, stablecoin_contract, receiver_address, expected_amount, currency, due_at, metadata)
         values ($1,$2,$3,$4,$5,$6,$7,$8)
         returning id::text, chain_slug, stablecoin_symbol, stablecoin_contract, receiver_address, expected_amount::text,
                   currency, due_at::text, paid_at::text, status, metadata, created_at::text`,
        [
          input.chain_slug,
          input.stablecoin_symbol,
          input.stablecoin_contract,
          input.receiver_address,
          input.expected_amount,
          input.currency,
          input.due_at,
          JSON.stringify(input.metadata ?? {})
        ]
      );
      return rows[0];
    }
  }
};
