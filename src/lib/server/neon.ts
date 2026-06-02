import "server-only";
import { Pool } from "pg";

let pool: Pool | null = null;

function getPool() {
  const connectionString = process.env.DATABASE_URL?.trim();
  if (!connectionString) throw new Error("DATABASE_URL is not configured.");

  if (!pool) {
    pool = new Pool({
      connectionString,
      ssl: connectionString.includes("sslmode=require")
        ? undefined
        : { rejectUnauthorized: false },
    });
  }

  return pool;
}

export async function query<T extends Record<string, unknown>>(
  sql: string,
  params: unknown[] = []
): Promise<T[]> {
  const result = await getPool().query(sql, params);
  return result.rows as T[];
}
