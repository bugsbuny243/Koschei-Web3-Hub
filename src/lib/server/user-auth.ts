import "server-only";
import { createHmac, timingSafeEqual } from "node:crypto";
import { cookies } from "next/headers";

const COOKIE_NAME = "koschei_member_session";

export type UserSession = { sub: string; email: string; expiresAt: number };
export class MemberSessionConfigurationError extends Error {}

export function assertMemberSessionConfigured() {
  const value = process.env.USER_SESSION_SECRET?.trim();
  if (!value) throw new MemberSessionConfigurationError("Auth session secret is not configured.");
  return value;
}

function sign(value: string) {
  return createHmac("sha256", assertMemberSessionConfigured()).update(value).digest("hex");
}

function safeEqual(left: string, right: string) {
  return left.length === right.length && timingSafeEqual(Buffer.from(left), Buffer.from(right));
}

function isValidEmail(email: string) {
  return /^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(email);
}

export async function getUserSession(): Promise<UserSession | null> {
  assertMemberSessionConfigured();
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
