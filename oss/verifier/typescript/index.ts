export interface VerdictLike {
  grade?: unknown;
  signed?: unknown;
  evidence?: unknown;
  rule_version?: unknown;
  triggered_rules?: unknown;
  decision_path?: unknown;
  signature?: unknown;
  signature_algorithm?: unknown;
  key_id?: unknown;
  payload_hash?: unknown;
}

export interface VerificationResult {
  valid: boolean;
  errors: string[];
}

export function verifySignedVerdict(value: unknown): VerificationResult {
  const errors: string[] = [];

  if (!value || typeof value !== "object" || Array.isArray(value)) {
    return { valid: false, errors: ["verdict must be an object"] };
  }

  const verdict = value as VerdictLike;

  if (typeof verdict.grade !== "string" || !/^[A-F-]$/.test(verdict.grade)) {
    errors.push("grade must be a single letter from A to F or '-' when no grade-changing rule was triggered");
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

  if (verdict.triggered_rules !== undefined) {
    if (!Array.isArray(verdict.triggered_rules) || verdict.triggered_rules.some(item => !isRuleHit(item))) {
      errors.push("triggered_rules must be an array of rule objects with rule_id and evidence_status");
    }
  }

  if (verdict.decision_path !== undefined) {
    if (
      !Array.isArray(verdict.decision_path) ||
      verdict.decision_path.some(item => typeof item !== "string" || item.trim() === "")
    ) {
      errors.push("decision_path must be an array of non-empty strings");
    }
  }

  return { valid: errors.length === 0, errors };
}

function isRuleHit(value: unknown): boolean {
  if (!value || typeof value !== "object" || Array.isArray(value)) return false;
  const rule = value as Record<string, unknown>;
  if (typeof rule.rule_id !== "string" || rule.rule_id.trim() === "") return false;
  return ["verified", "observed", "inferred", "unverified"].includes(String(rule.evidence_status));
}
