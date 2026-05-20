import { Pool } from "pg";

const pool = new Pool({ connectionString: process.env.DATABASE_URL });

export async function query<T = unknown>(text: string, params: unknown[] = []) {
  return pool.query<T>(text, params);
}
