import "server-only";
import { cookies } from "next/headers";

export type NeonAuthUser = { id: string; email: string };
type AuthPayload = { user?: unknown; data?: { user?: unknown } };
type HeadersWithSetCookie = Headers & { getSetCookie?: () => string[] };

function authBaseUrl() {
  const value = process.env.NEON_AUTH_BASE_URL?.trim().replace(/\/+$/, "");
  if (!value) throw new Error("NEON_AUTH_BASE_URL is not configured.");
  return value;
}

function extractUser(payload: AuthPayload): NeonAuthUser | null {
  const candidate = payload.user ?? payload.data?.user;
  if (!candidate || typeof candidate !== "object") return null;
  const { id, email } = candidate as { id?: unknown; email?: unknown };
  if (typeof id !== "string" || !id.trim() || typeof email !== "string" || !email.trim()) return null;
  return { id: id.trim(), email: email.trim().toLowerCase() };
}

function normalizeSetCookie(value: string) {
  return value.replace(/;\s*domain=[^;]+/gi, "").replace(/;\s*path=[^;]+/gi, "") + "; Path=/";
}

function responseCookies(headers: Headers) {
  const values = (headers as HeadersWithSetCookie).getSetCookie?.() ?? [];
  if (values.length) return values.map(normalizeSetCookie);
  const value = headers.get("set-cookie");
  return value ? [normalizeSetCookie(value)] : [];
}

async function cookieHeader() {
  return (await cookies()).getAll().map(({ name, value }) => `${name}=${value}`).join("; ");
}

async function request(path: string, init: RequestInit = {}) {
  const headers = new Headers(init.headers);
  headers.set("Accept", "application/json");
  const currentCookies = await cookieHeader();
  if (currentCookies) headers.set("Cookie", currentCookies);
  const response = await fetch(`${authBaseUrl()}${path}`, { ...init, headers, cache: "no-store" });
  const payload = await response.json().catch(() => ({})) as AuthPayload;
  if (!response.ok) throw new Error(`Neon Auth request failed (${response.status}).`);
  return { user: extractUser(payload), cookies: responseCookies(response.headers) };
}

export async function authenticateWithNeonAuth(mode: "login" | "signup", email: string, password: string) {
  const path = mode === "signup" ? "/sign-up/email" : "/sign-in/email";
  const result = await request(path, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ email, password }),
  });
  if (!result.user) throw new Error("Neon Auth response did not include a user.");
  return { ...result, user: result.user };
}

export async function getNeonAuthSession() {
  try { return (await request("/get-session")).user; } catch { return null; }
}

export async function signOutFromNeonAuth() {
  try { return (await request("/sign-out", { method: "POST" })).cookies; } catch { return []; }
}

export function appendNeonAuthCookies(response: Response, values: string[]) {
  for (const value of values) response.headers.append("Set-Cookie", value);
  return response;
}
