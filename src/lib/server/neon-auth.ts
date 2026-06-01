import "server-only";
import { createPublicKey, verify } from "node:crypto";
import { isValidEmail, normalizeEmail } from "@/lib/server/user-auth";

type JsonRecord = Record<string, unknown>;
type JwtHeader = { alg?: unknown; kid?: unknown };
type JwtClaims = { sub?: unknown; email?: unknown; exp?: unknown; nbf?: unknown; iss?: unknown };
type JsonWebKeyWithKid = JsonWebKey & { kid?: string };

export type NeonAuthIdentity = { sub: string; email: string };
export class NeonAuthConfigurationError extends Error {}
export class NeonAuthRequestError extends Error {
  constructor(message: string, public readonly status: number) { super(message); }
}

function authBaseUrl() {
  const value = (process.env.NEON_AUTH_BASE_URL || process.env.EXPO_PUBLIC_NEON_AUTH_URL || "").trim().replace(/\/+$/, "");
  if (!value) throw new NeonAuthConfigurationError("Auth service is not configured.");
  return value;
}

function asRecord(value: unknown): JsonRecord | null {
  return value && typeof value === "object" && !Array.isArray(value) ? value as JsonRecord : null;
}

function findToken(payload: unknown): string | null {
  const record = asRecord(payload);
  if (!record) return null;
  for (const key of ["access_token", "accessToken", "token", "session_token", "sessionToken", "jwt"]) {
    const value = record[key];
    if (typeof value === "string" && value.split(".").length === 3) return value;
  }
  for (const key of ["session", "data"]) {
    const token = findToken(record[key]);
    if (token) return token;
  }
  return null;
}

function parseJwtPart<T>(value: string): T {
  return JSON.parse(Buffer.from(value, "base64url").toString("utf8")) as T;
}

async function verifyWithJwks(header: JwtHeader, signingInput: string, signature: string) {
  const jwksUrl = process.env.NEON_AUTH_JWKS_URL?.trim();
  if (!jwksUrl) return;
  if (header.alg !== "RS256") throw new Error("Unsupported Neon Auth JWT algorithm.");
  const response = await fetch(jwksUrl, { headers: { Accept: "application/json" }, cache: "no-store" });
  if (!response.ok) throw new Error("Could not load Neon Auth signing keys.");
  const payload = await response.json() as { keys?: JsonWebKeyWithKid[] };
  const key = payload.keys?.find((candidate) => typeof header.kid === "string" && candidate.kid === header.kid)
    ?? (payload.keys?.length === 1 ? payload.keys[0] : undefined);
  if (!key) throw new Error("Neon Auth signing key was not found.");
  const valid = verify("RSA-SHA256", Buffer.from(signingInput), createPublicKey({ key, format: "jwk" }), Buffer.from(signature, "base64url"));
  if (!valid) throw new Error("Neon Auth JWT signature is invalid.");
}

async function identityFromToken(token: string): Promise<NeonAuthIdentity> {
  const [encodedHeader, encodedClaims, signature] = token.split(".");
  if (!encodedHeader || !encodedClaims || !signature) throw new Error("Neon Auth response did not include a valid JWT.");
  const header = parseJwtPart<JwtHeader>(encodedHeader);
  const claims = parseJwtPart<JwtClaims>(encodedClaims);
  await verifyWithJwks(header, `${encodedHeader}.${encodedClaims}`, signature);
  const now = Math.floor(Date.now() / 1000);
  if (typeof claims.exp !== "number" || claims.exp <= now) throw new Error("Neon Auth JWT has expired.");
  if (typeof claims.nbf === "number" && claims.nbf > now) throw new Error("Neon Auth JWT is not active yet.");
  const issuer = process.env.NEON_AUTH_ISSUER?.trim();
  if (issuer && claims.iss !== issuer) throw new Error("Neon Auth JWT issuer is invalid.");
  const email = normalizeEmail(claims.email);
  if (typeof claims.sub !== "string" || !claims.sub.trim() || !isValidEmail(email)) throw new Error("Neon Auth JWT is missing member claims.");
  return { sub: claims.sub.trim(), email };
}

export async function authenticateWithNeonAuth(mode: "login" | "signup", email: string, password: string) {
  const path = mode === "signup" ? "/sign-up/email" : "/sign-in/email";
  const body = mode === "signup" ? { email, password, name: email.split("@")[0] || "User" } : { email, password };
  const response = await fetch(`${authBaseUrl()}${path}`, {
    method: "POST",
    headers: { Accept: "application/json", "Content-Type": "application/json" },
    body: JSON.stringify(body),
    cache: "no-store",
  });
  const payload = await response.json().catch(() => ({})) as JsonRecord;
  if (!response.ok) {
    const message = payload.message ?? payload.error ?? `Neon Auth request failed (${response.status})`;
    throw new NeonAuthRequestError(String(message), response.status);
  }
  const token = response.headers.get("set-auth-jwt") || findToken(payload);
  if (!token) throw new Error("Neon Auth response did not include an auth token.");
  return identityFromToken(token);
}
