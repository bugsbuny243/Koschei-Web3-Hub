export interface VerdictLike {
  grade?: unknown;
  signed?: unknown;
  evidence?: unknown;
  rule_version?: unknown;
  triggered_rules?: unknown;
  decision_path?: unknown;
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
    errors.push("grade must be A through F or '-' when no grade-changing rule was triggered");
  }

  if (verdict.signed !== true) {
    errors.push("signed must be true");
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

  if (verdict.triggered_rules !== undefined && !Array.isArray(verdict.triggered_rules)) {
    errors.push("triggered_rules must be an array when present");
  }

  if (
    verdict.decision_path !== undefined &&
    (!Array.isArray(verdict.decision_path) || verdict.decision_path.some(item => typeof item !== "string" || item.trim() === ""))
  ) {
    errors.push("decision_path must be an array of non-empty strings when present");
  }

  return { valid: errors.length === 0, errors };
}
