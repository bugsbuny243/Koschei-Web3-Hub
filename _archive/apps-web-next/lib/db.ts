import { Pool } from "pg";
let pool: any = null;
export function getDbPool() {
  if (!process.env.DATABASE_URL) return null;
  if (!pool) pool = new Pool({ connectionString: process.env.DATABASE_URL });
  return pool;
}
