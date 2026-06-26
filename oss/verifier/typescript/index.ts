export interface VerdictLike {
  grade?: unknown;
  risk_index?: unknown;
  risk_level?: unknown;
  signed?: unknown;
  evidence?: unknown;
  rule_version?: unknown;
}

export interface VerificationResult {
  valid: boolean;
  errors: string[];
}

const levels = new Set(["low", "medium", "high", "critical"]);

export function verifySignedVerdict(value: unknown): VerificationResult {
  const errors: string[] = [];

  if (!value || typeof value !== "object") {
    return { valid: false, errors: ["verdict must be an object"] };
  }

  const verdict = value as VerdictLike;

  if (typeof verdict.grade !== "string" || !/^[A-F]$/.test(verdict.grade)) {
    errors.push("grade must be a single letter from A to F");
  }

  if (
    typeof verdict.risk_index !== "number" ||
    !Number.isFinite(verdict.risk_index) ||
    verdict.risk_index < 0 ||
    verdict.risk_index > 100
  ) {
    errors.push("risk_index must be a finite number from 0 to 100");
  }

  if (typeof verdict.risk_level !== "string" || !levels.has(verdict.risk_level)) {
    errors.push("risk_level must be low, medium, high or critical");
  }

  if (verdict.signed !== true) {
    errors.push("signed must be true");
  }

  if (verdict.evidence !== undefined) {
    if (!Array.isArray(verdict.evidence) || verdict.evidence.some(item => typeof item !== "string")) {
      errors.push("evidence must be an array of strings");
    }
  }

  if (verdict.rule_version !== undefined && typeof verdict.rule_version !== "string") {
    errors.push("rule_version must be a string when present");
  }

  return { valid: errors.length === 0, errors };
}
