import "server-only";
import { createHmac, timingSafeEqual } from "node:crypto";
import { cookies } from "next/headers";

const COOKIE_NAME = "koschei_member_session";
const SESSION_SECONDS = 60 * 60 * 24 * 7;

export type UserSession = { sub: string; email: string; expiresAt: number };

export function assertMemberSessionConfigured() {
  const value = process.env.USER_SESSION_SECRET?.trim();
  if (!value) throw new Error("USER_SESSION_SECRET is not configured. Set USER_SESSION_SECRET=long-random-secret.");
  return value;
}

function sign(value: string) {
  return createHmac("sha256", assertMemberSessionConfigured()).update(value).digest("hex");
}

function safeEqual(left: string, right: string) {
  return left.length === right.length && timingSafeEqual(Buffer.from(left), Buffer.from(right));
}

export function normalizeEmail(value: unknown) {
  return typeof value === "string" ? value.trim().toLowerCase() : "";
}

export function isValidEmail(email: string) {
  return /^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(email);
}

export function isValidPassword(password: unknown): password is string {
  return typeof password === "string" && password.length >= 8 && password.length <= 128;
}

export async function setUserCookie(sub: string, email: string) {
  const session: UserSession = { sub, email, expiresAt: Date.now() + SESSION_SECONDS * 1000 };
  const payload = Buffer.from(JSON.stringify(session)).toString("base64url");
  (await cookies()).set(COOKIE_NAME, `${payload}.${sign(payload)}`, {
    httpOnly: true,
    sameSite: "lax",
    secure: process.env.NODE_ENV === "production",
    path: "/",
    maxAge: SESSION_SECONDS,
  });
}

export async function clearUserCookie() {
  (await cookies()).delete(COOKIE_NAME);
}

export async function getUserSession(): Promise<UserSession | null> {
  const cookieStore = await cookies();
  const value = cookieStore.get(COOKIE_NAME)?.value;
  if (!value) return null;
  const [payload, signature] = value.split(".");
  if (!payload || !signature || !safeEqual(sign(payload), signature)) return null;
  try {
    const session = JSON.parse(Buffer.from(payload, "base64url").toString("utf8")) as UserSession;
    if (typeof session.sub !== "string" || !session.sub.trim() || !isValidEmail(session.email) || session.expiresAt <= Date.now()) return null;
    return session;
  } catch {
    return null;
  }
}
