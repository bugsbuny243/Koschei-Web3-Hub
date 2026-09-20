import { createHash, createPublicKey, verify as verifyEd25519 } from "node:crypto";

export const VERDICT_SIGNATURE_DOMAIN = "koschei.arvis-verdict/v1";

export interface VerdictLike {
  target?: unknown;
  network?: unknown;
  grade?: unknown;
  verdict?: unknown;
  signed?: unknown;
  evidence?: unknown;
  rule_version?: unknown;
  triggered_rules?: unknown;
  watch_flags?: unknown;
  decision_path?: unknown;
  signature?: unknown;
  signature_algorithm?: unknown;
  key_id?: unknown;
  payload_hash?: unknown;
  created_at?: unknown;
}

export type TrustedKeyRegistry = Readonly<Record<string, string>>;

export interface VerificationResult {
  valid: boolean;
  errors: string[];
}

type CanonicalRuleBinding = {
  rule_id: string;
  evidence_status: string;
  title: string;
  tier: string;
  grade_effect: string;
  grade_cap: string;
  count: number;
  summary: string;
  evidence_keys: string[];
  signatures: string[];
};

type CanonicalVerdictPayload = {
  schema_version: typeof VERDICT_SIGNATURE_DOMAIN;
  target: string;
  network: string;
  grade: string;
  verdict: string;
  evidence: string[];
  rule_version: string;
  triggered_rules: CanonicalRuleBinding[];
  watch_flags: CanonicalRuleBinding[];
  decision_path: string[];
  created_at: string;
};

const ED25519_SPKI_PREFIX = Buffer.from("302a300506032b6570032100", "hex");

export function verifySignedVerdict(
  value: unknown,
  trustedKeys: TrustedKeyRegistry = {},
): VerificationResult {
  const errors = validateVerdictShape(value);
  if (errors.length > 0) {
    return { valid: false, errors };
  }

  const verdict = value as VerdictLike;
  const keyID = String(verdict.key_id);
  const trustedKey = trustedKeys[keyID];
  if (typeof trustedKey !== "string" || trustedKey.trim() === "") {
    errors.push(`no trusted Ed25519 public key is configured for key_id "${keyID}"`);
    return { valid: false, errors };
  }

  const payload = canonicalVerdictPayloadBytes(verdict);
  const expectedHash = `sha256:${createHash("sha256").update(payload).digest("hex")}`;
  if (verdict.payload_hash !== expectedHash) {
    errors.push("payload_hash does not match the canonical authenticated verdict payload");
  }

  const publicKeyBytes = decodeCanonicalBase64URL(trustedKey, 32);
  if (!publicKeyBytes) {
    errors.push("trusted Ed25519 public key must be canonical unpadded base64url for 32 bytes");
  }

  const signatureBytes = decodeCanonicalBase64URL(String(verdict.signature), 64);
  if (!signatureBytes) {
    errors.push("signature must be canonical unpadded base64url for a 64-byte Ed25519 signature");
  }

  if (publicKeyBytes && signatureBytes) {
    try {
      const publicKey = createPublicKey({
        key: Buffer.concat([ED25519_SPKI_PREFIX, publicKeyBytes]),
        format: "der",
        type: "spki",
      });
      if (!verifyEd25519(null, payload, publicKey, signatureBytes)) {
        errors.push("signature did not verify against the trusted Ed25519 key");
      }
    } catch {
      errors.push("trusted Ed25519 public key could not be loaded");
    }
  }

  return { valid: errors.length === 0, errors };
}

export function canonicalVerdictPayloadBytes(value: VerdictLike): Buffer {
  const payload: CanonicalVerdictPayload = {
    schema_version: VERDICT_SIGNATURE_DOMAIN,
    target: String(value.target),
    network: String(value.network),
    grade: String(value.grade),
    verdict: String(value.verdict),
    evidence: (value.evidence as string[]).map(item => item.trim()),
    rule_version: String(value.rule_version).trim(),
    triggered_rules: canonicalRuleBindings(value.triggered_rules),
    watch_flags: canonicalRuleBindings(value.watch_flags),
    decision_path: Array.isArray(value.decision_path)
      ? (value.decision_path as string[]).map(item => item.trim())
      : [],
    created_at: String(value.created_at),
  };
  return Buffer.from(goCompatibleJSONStringify(payload), "utf8");
}

function goCompatibleJSONStringify(value: unknown): string {
  const encoded = JSON.stringify(value);
  if (encoded === undefined) {
    throw new Error("canonical verdict payload could not be encoded");
  }
  return encoded.replace(/[<>&\u2028\u2029]/g, character => {
    switch (character) {
      case "<":
        return "\\u003c";
      case ">":
        return "\\u003e";
      case "&":
        return "\\u0026";
      case "\u2028":
        return "\\u2028";
      case "\u2029":
        return "\\u2029";
      default:
        return character;
    }
  });
}

function validateVerdictShape(value: unknown): string[] {
  const errors: string[] = [];

  if (!value || typeof value !== "object" || Array.isArray(value)) {
    return ["verdict must be an object"];
  }

  const verdict = value as VerdictLike;

  if (typeof verdict.target !== "string" || verdict.target.trim() === "") {
    errors.push("target is required");
  }

  if (typeof verdict.network !== "string" || verdict.network.trim() === "") {
    errors.push("network is required");
  }

  if (typeof verdict.grade !== "string" || !/^[A-F-]$/.test(verdict.grade)) {
    errors.push("grade must be a single letter from A to F or '-' when no grade-changing rule was triggered");
  }

  if (typeof verdict.verdict !== "string" || verdict.verdict.trim() === "") {
    errors.push("verdict is required");
  }

  if (verdict.signed !== true) {
    errors.push("signed must be true");
  }

  if (typeof verdict.signature !== "string" || verdict.signature.trim() === "") {
    errors.push("signature is required when signed is true");
  }

  if (verdict.signature_algorithm !== "ed25519") {
    errors.push("signature_algorithm must be ed25519");
  }

  if (typeof verdict.key_id !== "string" || verdict.key_id.trim() === "") {
    errors.push("key_id is required");
  }

  if (typeof verdict.payload_hash !== "string" || !/^sha256:[0-9a-f]{64}$/.test(verdict.payload_hash)) {
    errors.push("payload_hash must be sha256:<64 lowercase hex chars>");
  }

  if (
    !Array.isArray(verdict.evidence) ||
    verdict.evidence.length === 0 ||
    verdict.evidence.some(item => typeof item !== "string" || item.trim() === "")
  ) {
    errors.push("evidence must be a non-empty array of strings");
  }

  if (typeof verdict.rule_version !== "string" || verdict.rule_version.trim() === "") {
    errors.push("rule_version is required");
  }

  if (verdict.triggered_rules !== undefined && !validRuleArray(verdict.triggered_rules)) {
    errors.push("triggered_rules must be an array of rule objects with rule_id and evidence_status");
  }

  if (verdict.watch_flags !== undefined && !validRuleArray(verdict.watch_flags)) {
    errors.push("watch_flags must be an array of rule objects with rule_id and evidence_status");
  }

  if (
    verdict.decision_path !== undefined &&
    (!Array.isArray(verdict.decision_path) ||
      verdict.decision_path.some(item => typeof item !== "string" || item.trim() === ""))
  ) {
    errors.push("decision_path must be an array of non-empty strings");
  }

  if (
    typeof verdict.created_at !== "string" ||
    verdict.created_at.trim() === "" ||
    Number.isNaN(Date.parse(verdict.created_at))
  ) {
    errors.push("created_at must be a valid date-time string");
  }

  return errors;
}

function validRuleArray(value: unknown): boolean {
  return Array.isArray(value) && value.every(item => isRuleHit(item));
}

function isRuleHit(value: unknown): boolean {
  if (!value || typeof value !== "object" || Array.isArray(value)) return false;
  const rule = value as Record<string, unknown>;
  if (typeof rule.rule_id !== "string" || rule.rule_id.trim() === "") return false;
  if (!["verified", "observed", "inferred", "unverified"].includes(String(rule.evidence_status))) return false;
  for (const key of ["evidence_keys", "signatures"]) {
    if (rule[key] !== undefined && !isStringArray(rule[key])) return false;
  }
  if (rule.count !== undefined && (typeof rule.count !== "number" || !Number.isInteger(rule.count) || rule.count < 0)) return false;
  return true;
}

function canonicalRuleBindings(value: unknown): CanonicalRuleBinding[] {
  if (!Array.isArray(value)) return [];
  return value.map(item => {
    const rule = item as Record<string, unknown>;
    return {
      rule_id: stringValue(rule.rule_id),
      evidence_status: stringValue(rule.evidence_status),
      title: stringValue(rule.title),
      tier: stringValue(rule.tier),
      grade_effect: stringValue(rule.grade_effect),
      grade_cap: stringValue(rule.grade_cap),
      count: typeof rule.count === "number" && Number.isInteger(rule.count) ? rule.count : 0,
      summary: stringValue(rule.summary),
      evidence_keys: stringArray(rule.evidence_keys),
      signatures: stringArray(rule.signatures),
    };
  });
}

function stringValue(value: unknown): string {
  return typeof value === "string" ? value.trim() : "";
}

function stringArray(value: unknown): string[] {
  if (!Array.isArray(value)) return [];
  return value.map(item => String(item).trim());
}

function isStringArray(value: unknown): boolean {
  return Array.isArray(value) && value.every(item => typeof item === "string" && item.trim() !== "");
}

function decodeCanonicalBase64URL(value: string, expectedBytes: number): Buffer | null {
  if (!/^[A-Za-z0-9_-]+$/.test(value)) return null;
  try {
    const decoded = Buffer.from(value, "base64url");
    if (decoded.length !== expectedBytes || decoded.toString("base64url") !== value) return null;
    return decoded;
  } catch {
    return null;
  }
}
