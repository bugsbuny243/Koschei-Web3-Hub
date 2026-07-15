export * from "./client.js";

export type VerdictGrade = "A" | "B" | "C" | "D" | "E" | "F" | "-";
export type EvidenceStatus = "verified" | "observed" | "inferred" | "unverified";
export type RiskDecision = "block" | "warn" | "allow" | "withhold";

export interface TriggeredRule {
  rule_id: string;
  title?: string;
  tier?: string;
  evidence_status: EvidenceStatus;
  grade_effect?: string;
  grade_cap?: VerdictGrade;
  count?: number;
  summary?: string;
  evidence_keys?: string[];
  signatures?: string[];
  facts?: Record<string, unknown>;
  [key: string]: unknown;
}

export interface SignedVerdict {
  target?: string;
  network?: string;
  grade: VerdictGrade;
  verdict?: string;
  recommendation?: string;
  evidence: string[];
  rule_version: string;
  triggered_rules?: TriggeredRule[];
  decision_path?: string[];
  signed: true;
  signature?: string;
  created_at?: string;
  [key: string]: unknown;
}

export interface VerdictValidationResult {
  ok: boolean;
  errors: string[];
  verdict?: SignedVerdict;
}

export interface RiskPolicy {
  blockGrades?: VerdictGrade[];
  warnGrades?: VerdictGrade[];
}

export interface RiskPolicyResult {
  decision: RiskDecision;
  reason: string;
  grade?: VerdictGrade;
  triggeredRules?: TriggeredRule[];
  validation: VerdictValidationResult;
}

export function validateSignedVerdict(value: unknown): VerdictValidationResult {
  if (!value || typeof value !== "object" || Array.isArray(value)) {
    return { ok: false, errors: ["verdict must be an object"] };
  }

  const candidate = value as Record<string, unknown>;
  const errors: string[] = [];

  if (candidate.signed !== true) errors.push("signed must be true");

  if (typeof candidate.grade !== "string" || !/^[A-F-]$/.test(candidate.grade)) {
    errors.push("grade must be A through F or '-' when no grade-changing rule was triggered");
  }

  if (
    !Array.isArray(candidate.evidence) ||
    candidate.evidence.length === 0 ||
    candidate.evidence.some((item) => typeof item !== "string" || item.trim() === "")
  ) {
    errors.push("evidence must be a non-empty array of strings");
  }

  if (typeof candidate.rule_version !== "string" || candidate.rule_version.trim() === "") {
    errors.push("rule_version is required");
  }

  if (candidate.triggered_rules !== undefined) {
    if (!Array.isArray(candidate.triggered_rules) || candidate.triggered_rules.some((item) => !isTriggeredRule(item))) {
      errors.push("triggered_rules must be an array of rule objects with rule_id and evidence_status");
    }
  }

  if (candidate.decision_path !== undefined) {
    if (
      !Array.isArray(candidate.decision_path) ||
      candidate.decision_path.some((item) => typeof item !== "string" || item.trim() === "")
    ) {
      errors.push("decision_path must be an array of non-empty strings");
    }
  }

  return errors.length === 0
    ? { ok: true, errors: [], verdict: candidate as unknown as SignedVerdict }
    : { ok: false, errors };
}

export function isSignedVerdict(value: unknown): value is SignedVerdict {
  return validateSignedVerdict(value).ok;
}

export function evaluateVerdictPolicy(value: unknown, policy: RiskPolicy = {}): RiskPolicyResult {
  const validation = validateSignedVerdict(value);
  if (!validation.ok || !validation.verdict) {
    return {
      decision: "withhold",
      reason: validation.errors.join("; ") || "Signed verdict unavailable.",
      validation
    };
  }

  const verdict = validation.verdict;
  const grade = verdict.grade;
  const triggeredRules = verdict.triggered_rules ?? [];

  if (grade === "-") {
    return {
      decision: "withhold",
      reason: "No grade-changing rule was triggered; absence of evidence is not an A grade.",
      grade,
      triggeredRules,
      validation
    };
  }

  const blockGrades = new Set(policy.blockGrades ?? ["D", "E", "F"]);
  const warnGrades = new Set(policy.warnGrades ?? ["B", "C"]);

  if (blockGrades.has(grade)) {
    return {
      decision: "block",
      reason: `Grade ${grade} matches the configured blocking grades.`,
      grade,
      triggeredRules,
      validation
    };
  }

  if (warnGrades.has(grade)) {
    return {
      decision: "warn",
      reason: `Grade ${grade} matches the configured warning grades.`,
      grade,
      triggeredRules,
      validation
    };
  }

  return {
    decision: "allow",
    reason: `Grade ${grade} does not match a configured blocking or warning grade.`,
    grade,
    triggeredRules,
    validation
  };
}

function isTriggeredRule(value: unknown): value is TriggeredRule {
  if (!value || typeof value !== "object" || Array.isArray(value)) return false;
  const rule = value as Record<string, unknown>;
  if (typeof rule.rule_id !== "string" || rule.rule_id.trim() === "") return false;
  return ["verified", "observed", "inferred", "unverified"].includes(String(rule.evidence_status));
}
