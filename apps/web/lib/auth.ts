import crypto from "node:crypto";
import { getDbPool } from "@/lib/db";

const JWT_SECRET = process.env.TOGETHER_API_KEY;
const TOKEN_TTL_SECONDS = 60 * 60 * 24 * 7;

export type AppUser = {
  id: number;
  email: string;
  credits: number;
};

function base64url(input: Buffer | string) {
  return Buffer.from(input)
    .toString("base64")
    .replace(/=/g, "")
    .replace(/\+/g, "-")
    .replace(/\//g, "_");
}

function sign(payload: Record<string, unknown>) {
  if (!JWT_SECRET) throw new Error("TOGETHER_API_KEY is required for JWT signing");
  const header = base64url(JSON.stringify({ alg: "HS256", typ: "JWT" }));
  const body = base64url(JSON.stringify(payload));
  const signature = base64url(crypto.createHmac("sha256", JWT_SECRET).update(`${header}.${body}`).digest());
  return `${header}.${body}.${signature}`;
}

export function createToken(user: AppUser) {
  const now = Math.floor(Date.now() / 1000);
  return sign({ sub: user.id, email: user.email, credits: user.credits, iat: now, exp: now + TOKEN_TTL_SECONDS });
}

export function verifyToken(token: string) {
  if (!JWT_SECRET) return null;
  const [header, body, signature] = token.split(".");
  if (!header || !body || !signature) return null;
  const expected = base64url(crypto.createHmac("sha256", JWT_SECRET).update(`${header}.${body}`).digest());
  if (signature !== expected) return null;
  const payload = JSON.parse(Buffer.from(body, "base64").toString("utf-8"));
  if (!payload.exp || payload.exp < Math.floor(Date.now() / 1000)) return null;
  return payload as { sub: number; email: string; credits: number; exp: number };
}

export async function ensureUsersTable() {
  const db = getDbPool();
  if (!db) throw new Error("DATABASE_URL is not configured");
  await db.query(`
    CREATE TABLE IF NOT EXISTS app_users (
      id SERIAL PRIMARY KEY,
      email TEXT UNIQUE NOT NULL,
      password_hash TEXT NOT NULL,
      credits INTEGER NOT NULL DEFAULT 100,
      created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
    )
  `);
  return db;
}

export function hashPassword(password: string) {
  const salt = crypto.randomBytes(16).toString("hex");
  const hash = crypto.scryptSync(password, salt, 64).toString("hex");
  return `${salt}:${hash}`;
}

export function validatePassword(password: string, stored: string) {
  const [salt, originalHash] = stored.split(":");
  if (!salt || !originalHash) return false;
  const hash = crypto.scryptSync(password, salt, 64).toString("hex");
  return crypto.timingSafeEqual(Buffer.from(hash), Buffer.from(originalHash));
}

export async function getUserFromAuthHeader(authHeader: string | null) {
  if (!authHeader?.startsWith("Bearer ")) return null;
  const payload = verifyToken(authHeader.replace("Bearer ", ""));
  if (!payload) return null;
  return payload;
}
