import "server-only";
import { getNeonAuthSession } from "@/lib/server/neon-auth";

export function normalizeEmail(value: unknown) {
  return typeof value === "string" ? value.trim().toLowerCase() : "";
}

export function isValidEmail(email: string) {
  return /^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(email);
}

export function isValidPassword(password: unknown): password is string {
  return typeof password === "string" && password.length >= 8 && password.length <= 128;
}

export async function getUserSession() {
  const user = await getNeonAuthSession();
  return user ? { email: user.email } : null;
}
