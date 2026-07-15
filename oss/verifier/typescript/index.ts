export interface VerdictRuleLike {
  rule_id?: unknown;
  evidence_status?: unknown;
}

export interface VerdictLike {
  grade?: unknown;
  signed?: unknown;
  signature?: unknown;
  evidence?: unknown;
  rule_version?: unknown;
  triggered_rules?: unknown;
  decision_path?: unknown;
}

export interface VerificationResult {
  valid: boolean;
  errors: string[];
}

const evidenceStatuses = new Set(["verified", "observed", "inferred", "unverified"]);

export function verifySignedVerdict(value: unknown): VerificationResult {
  const errors: string[] = [];

  if (!value || typeof value !== "object" || Array.isArray(value)) {
    return { valid: false, errors: ["verdict must be an object"] };
  }

  const verdict = value as VerdictLike;

  if (typeof verdict.grade !== "string" || !/^[A-F-]$/.test(verdict.grade)) {
    errors.push("grade must be A through F or '-' when no grade-changing rule was triggered");
  }

  if (typeof verdict.signed !== "boolean") {
    errors.push("signed must be a boolean");
  } else if (verdict.signed && (typeof verdict.signature !== "string" || verdict.signature.trim() === "")) {
    errors.push("signature is required when signed is true");
  }

  if (!Array.isArray(verdict.evidence) || verdict.evidence.some(item => typeof item !== "string" || item.trim() === "")) {
    errors.push("evidence must be an array of non-empty strings");
  }

  if (typeof verdict.rule_version !== "string" || verdict.rule_version.trim() === "") {
    errors.push("rule_version is required");
  }

  if (verdict.triggered_rules !== undefined) {
    if (!Array.isArray(verdict.triggered_rules)) {
      errors.push("triggered_rules must be an array when present");
    } else {
      verdict.triggered_rules.forEach((item, index) => {
        if (!item || typeof item !== "object" || Array.isArray(item)) {
          errors.push(`triggered_rules[${index}] must be an object`);
          return;
        }
        const rule = item as VerdictRuleLike;
        if (typeof rule.rule_id !== "string" || rule.rule_id.trim() === "") {
          errors.push(`triggered_rules[${index}].rule_id is required`);
        }
        if (typeof rule.evidence_status !== "string" || !evidenceStatuses.has(rule.evidence_status)) {
          errors.push(`triggered_rules[${index}].evidence_status is invalid`);
        }
      });
    }
  }

  if (
    verdict.decision_path !== undefined &&
    (!Array.isArray(verdict.decision_path) || verdict.decision_path.some(item => typeof item !== "string" || item.trim() === ""))
  ) {
    errors.push("decision_path must be an array of non-empty strings when present");
  }

  return { valid: errors.length === 0, errors };
}
