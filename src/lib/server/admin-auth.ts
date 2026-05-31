import "server-only";
import { createHmac, timingSafeEqual } from "node:crypto";
import { cookies } from "next/headers";

const COOKIE_NAME = "koschei_admin";
function signature() { return createHmac("sha256", process.env.ADMIN_PASSWORD || "unconfigured").update(process.env.ADMIN_EMAIL || "unconfigured").digest("hex"); }
export function isValidAdminCredentials(email: unknown, password: unknown) { return Boolean(process.env.ADMIN_EMAIL && process.env.ADMIN_PASSWORD && email === process.env.ADMIN_EMAIL && password === process.env.ADMIN_PASSWORD); }
export async function isAdminRequest() { const value = (await cookies()).get(COOKIE_NAME)?.value; if (!value) return false; const expected = signature(); return value.length === expected.length && timingSafeEqual(Buffer.from(value), Buffer.from(expected)); }
export async function setAdminCookie() { (await cookies()).set(COOKIE_NAME, signature(), { httpOnly: true, sameSite: "strict", secure: process.env.NODE_ENV === "production", path: "/", maxAge: 60 * 60 * 8 }); }
export async function clearAdminCookie() { (await cookies()).delete(COOKIE_NAME); }
