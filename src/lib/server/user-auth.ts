import "server-only";
import { createHmac, randomBytes, scryptSync, timingSafeEqual } from "node:crypto";
import { cookies } from "next/headers";

const COOKIE_NAME = "koschei_member_session";
const SESSION_SECONDS = 60 * 60 * 24 * 7;

type UserSession = { email: string; expiresAt: number };

export function assertMemberSessionConfigured() {
  const value = process.env.USER_SESSION_SECRET || process.env.MEMBER_SESSION_SECRET;
  if (!value) throw new Error("USER_SESSION_SECRET or MEMBER_SESSION_SECRET is not configured.");
  return value;
}

function sessionSecret() {
  return assertMemberSessionConfigured();
}

function sign(value: string) {
  return createHmac("sha256", sessionSecret()).update(value).digest("hex");
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

export function hashPassword(password: string) {
  const salt = randomBytes(16).toString("hex");
  return `${salt}:${scryptSync(password, salt, 64).toString("hex")}`;
}

export function verifyPassword(password: string, storedHash: string) {
  const [salt, expected] = storedHash.split(":");
  if (!salt || !expected) return false;
  const actual = scryptSync(password, salt, 64).toString("hex");
  return safeEqual(actual, expected);
}

export async function setUserCookie(email: string) {
  const session: UserSession = { email, expiresAt: Date.now() + SESSION_SECONDS * 1000 };
  const payload = Buffer.from(JSON.stringify(session)).toString("base64url");
  (await cookies()).set(COOKIE_NAME, `${payload}.${sign(payload)}`, {
    httpOnly: true,
    sameSite: "lax",
    secure: true,
    path: "/",
    maxAge: SESSION_SECONDS,
  });
}

export async function clearUserCookie() {
  (await cookies()).delete(COOKIE_NAME);
}

export async function getUserSession(): Promise<UserSession | null> {
  const cookieStore = await cookies();
  assertMemberSessionConfigured();
  const value = cookieStore.get(COOKIE_NAME)?.value;
  if (!value) return null;
  const [payload, signature] = value.split(".");
  if (!payload || !signature || !safeEqual(sign(payload), signature)) return null;
  try {
    const session = JSON.parse(Buffer.from(payload, "base64url").toString("utf8")) as UserSession;
    if (!isValidEmail(session.email) || session.expiresAt <= Date.now()) return null;
    return session;
  } catch {
    return null;
  }
}
