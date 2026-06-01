import "server-only";
import { createPublicKey, verify } from "node:crypto";
import { isValidEmail, normalizeEmail } from "@/lib/server/user-auth";

type JsonRecord = Record<string, unknown>;
type JwtHeader = { alg?: unknown; kid?: unknown };
type JwtClaims = { sub?: unknown; email?: unknown; exp?: unknown; nbf?: unknown; iss?: unknown };
type JsonWebKeyWithKid = JsonWebKey & { kid?: string };
type NeonAuthMode = "login" | "signup";

export type NeonAuthIdentity = { sub: string; email: string };
export class NeonAuthConfigurationError extends Error {}
export class NeonAuthProviderError extends Error {}
export class NeonAuthSessionError extends Error {}
export class NeonAuthVerificationError extends Error {}
export class NeonAuthRequestError extends Error {
  constructor(message: string, public readonly status: number) { super(message); }
}
export class NeonAuthProviderRequestError extends Error {}
export class NeonAuthSessionError extends Error {}

function envPresence() {
  return {
    NEON_AUTH_BASE_URL: Boolean(process.env.NEON_AUTH_BASE_URL?.trim()),
    EXPO_PUBLIC_NEON_AUTH_URL: Boolean(process.env.EXPO_PUBLIC_NEON_AUTH_URL?.trim()),
    NEON_AUTH_ISSUER: Boolean(process.env.NEON_AUTH_ISSUER?.trim()),
    NEON_AUTH_JWKS_URL: Boolean(process.env.NEON_AUTH_JWKS_URL?.trim()),
  };
}

function sanitizeForLog(value: unknown, depth = 0): unknown {
  if (depth > 4) return "[truncated]";
  if (Array.isArray(value)) return value.slice(0, 20).map((item) => sanitizeForLog(item, depth + 1));
  const record = asRecord(value);
  if (!record) return typeof value === "string" && value.length > 500 ? `${value.slice(0, 500)}…` : value;
  return Object.fromEntries(Object.entries(record).map(([key, item]) => [
    key,
    /token|jwt|password|secret|authorization|cookie/i.test(key) ? "[redacted]" : sanitizeForLog(item, depth + 1),
  ]));
}

function logAuthIssue(event: string, details: JsonRecord = {}) {
  console.error(`[neon-auth] ${event}`, { ...details, env: envPresence() });
}

function authBaseUrl() {
  const value = (process.env.NEON_AUTH_BASE_URL || process.env.EXPO_PUBLIC_NEON_AUTH_URL || "").trim().replace(/\/+$/, "");
  if (!value) {
    logAuthIssue("auth base URL is not configured");
    throw new NeonAuthConfigurationError("Auth service is not configured.");
  }
  return value;
}

function asRecord(value: unknown): JsonRecord | null {
  return value && typeof value === "object" && !Array.isArray(value) ? value as JsonRecord : null;
}

function stringAtPath(payload: unknown, path: string[]) {
  let value = payload;
  for (const key of path) value = asRecord(value)?.[key];
  return typeof value === "string" && value.split(".").length === 3 ? value : null;
}

function findToken(payload: unknown): string | null {
  const paths = [
    ["data", "session", "access_token"],
    ["session", "access_token"],
    ["data", "access_token"],
    ["access_token"],
    ["data", "token"],
    ["token"],
    ["data", "session", "token"],
    ["session", "token"],
  ];
  for (const path of paths) {
    const token = stringAtPath(payload, path);
    if (token) return token;
  }
  return null;
}

function parseJwtPart<T>(value: string): T {
  return JSON.parse(Buffer.from(value, "base64url").toString("utf8")) as T;
}

function tokenHeader(token: string): JwtHeader {
  const [encodedHeader] = token.split(".");
  if (!encodedHeader) return {};
  try { return parseJwtPart<JwtHeader>(encodedHeader); } catch { return {}; }
}

function jwtDetails(header: JwtHeader) {
  return {
    alg: typeof header.alg === "string" ? header.alg : "[missing]",
    kid: typeof header.kid === "string" ? header.kid : "[missing]",
  };
}

function normalizedUrlCandidates(value: string | undefined, includeJwksBase = false) {
  const trimmed = value?.trim().replace(/\/+$/, "");
  if (!trimmed) return [];
  const candidates = new Set(includeJwksBase ? [] : [trimmed]);
  try {
    const url = new URL(trimmed);
    candidates.add(url.origin.replace(/\/+$/, ""));
    if (includeJwksBase) {
      const basePath = url.pathname.replace(/\/(?:\.well-known\/)?(?:jwks(?:\.json)?|openid-configuration)$/i, "").replace(/\/+$/, "");
      candidates.add(`${url.origin}${basePath}`.replace(/\/+$/, ""));
    }
  } catch { /* An explicit non-URL issuer remains a valid exact candidate. */ }
  return [...candidates];
}

function allowedIssuers() {
  return new Set([
    ...normalizedUrlCandidates(process.env.NEON_AUTH_ISSUER),
    ...normalizedUrlCandidates(process.env.NEON_AUTH_BASE_URL),
    ...normalizedUrlCandidates(process.env.EXPO_PUBLIC_NEON_AUTH_URL),
    ...normalizedUrlCandidates(process.env.NEON_AUTH_JWKS_URL, true),
  ]);
}

async function verifyWithJwks(header: JwtHeader, signingInput: string, signature: string) {
  if (header.alg !== "RS256" && header.alg !== "EdDSA") throw new Error("Unsupported Neon Auth JWT algorithm.");
  const jwksUrl = process.env.NEON_AUTH_JWKS_URL?.trim();
  if (!jwksUrl) {
    logAuthIssue("JWKS URL is not configured", jwtDetails(header));
    throw new NeonAuthConfigurationError("Auth service is not configured.");
  }
  const response = await fetch(jwksUrl, { headers: { Accept: "application/json" }, cache: "no-store" });
  if (!response.ok) throw new Error("Could not load Neon Auth signing keys.");
  const payload = await response.json() as { keys?: JsonWebKeyWithKid[] };
  const key = payload.keys?.find((candidate) => typeof header.kid === "string" && candidate.kid === header.kid)
    ?? (payload.keys?.length === 1 ? payload.keys[0] : undefined);
  if (!key) throw new Error("Neon Auth signing key was not found.");
  const publicKey = createPublicKey({ key, format: "jwk" });
  const valid = header.alg === "RS256"
    ? verify("RSA-SHA256", Buffer.from(signingInput), publicKey, Buffer.from(signature, "base64url"))
    : verify(null, Buffer.from(signingInput), publicKey, Buffer.from(signature, "base64url"));
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
  const issuers = allowedIssuers();
  const issuer = typeof claims.iss === "string" ? claims.iss.trim().replace(/\/+$/, "") : "";
  if (issuers.size > 0 && !issuers.has(issuer)) throw new Error("Neon Auth JWT issuer is invalid.");
  const email = normalizeEmail(claims.email);
  if (typeof claims.sub !== "string" || !claims.sub.trim() || !isValidEmail(email)) throw new Error("Neon Auth JWT is missing member claims.");
  return { sub: claims.sub.trim(), email };
}

export function isDuplicateSignupError(error: NeonAuthRequestError) {
  return error.status === 409 || /duplicate|already exists|user already exists|email already registered/i.test(error.message);
}

export function isCredentialRejection(error: NeonAuthRequestError) {
  return error.status >= 400 && error.status < 500 && error.status !== 429;
}

export async function authenticateWithNeonAuth(mode: NeonAuthMode, email: string, password: string) {
  const path = mode === "signup" ? "/sign-up/email" : "/sign-in/email";
  const body = mode === "signup" ? { email, password, name: email.split("@")[0] || "User" } : { email, password };
  let response: Response;
  try {
    response = await fetch(`${authBaseUrl()}${path}`, {
      method: "POST",
      headers: { Accept: "application/json", "Content-Type": "application/json" },
      body: JSON.stringify(body),
      cache: "no-store",
    });
  } catch (error) {
    if (error instanceof NeonAuthConfigurationError) throw error;
    logAuthIssue("provider request failed", { mode, error: error instanceof Error ? error.message : "Unknown provider request failure." });
    throw new NeonAuthProviderError("Auth provider request failed.");
  }
  const payload = await response.json().catch(() => ({})) as JsonRecord;
  if (!response.ok) {
    logAuthIssue("provider rejected request", { mode, providerStatus: response.status, providerResponse: sanitizeForLog(payload) });
    const message = payload.message ?? payload.error ?? `Neon Auth request failed (${response.status})`;
    throw new NeonAuthRequestError(String(message), response.status);
  }
  const token = response.headers.get("set-auth-jwt") || findToken(payload);
  if (!token) {
    logAuthIssue("provider did not return a session", { mode, providerStatus: response.status, providerResponse: sanitizeForLog(payload) });
    throw new NeonAuthSessionError("Auth provider did not return a session.");
  }
  const header = tokenHeader(token);
  try {
    return await identityFromToken(token);
  } catch (error) {
    logAuthIssue("token verification failed", { mode, providerStatus: response.status, ...jwtDetails(header), error: error instanceof Error ? error.message : "Unknown verification failure." });
    if (error instanceof NeonAuthConfigurationError) throw error;
    throw new NeonAuthVerificationError("Auth token verification failed.");
  }
}
