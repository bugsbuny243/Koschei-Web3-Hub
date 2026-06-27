export type ArvisSource = "pump" | "raydium" | "unknown";
export type ArvisTargetType = "token" | "pool" | "wallet" | "program" | "transaction" | "unknown";

export interface RawObservation {
  module_id?: unknown;
  source?: unknown;
  event_type?: unknown;
  signature?: unknown;
  target?: unknown;
  target_mint?: unknown;
  mint?: unknown;
  address?: unknown;
  target_type?: unknown;
  network?: unknown;
  created_at?: unknown;
  observed_at?: unknown;
  metadata?: unknown;
  [key: string]: unknown;
}

export interface NormalizedObservation {
  schema_version: "1.0";
  source: ArvisSource;
  module_id: string;
  event_type: string;
  signature?: string;
  target: string;
  target_type: ArvisTargetType;
  network: string;
  observed_at: string;
  metadata: Record<string, unknown>;
}

export interface NormalizeResult {
  ok: boolean;
  observation?: NormalizedObservation;
  errors: string[];
}

const asString = (value: unknown): string =>
  typeof value === "string" ? value.trim() : "";

const firstString = (...values: unknown[]): string => {
  for (const value of values) {
    const normalized = asString(value);
    if (normalized) return normalized;
  }
  return "";
};

const normalizeSource = (moduleId: string, source: string): ArvisSource => {
  const haystack = `${moduleId} ${source}`.toLowerCase();
  if (haystack.includes("pump")) return "pump";
  if (haystack.includes("raydium")) return "raydium";
  return "unknown";
};

const normalizeTargetType = (value: string, source: ArvisSource): ArvisTargetType => {
  const normalized = value.toLowerCase();
  if (["token", "mint"].includes(normalized)) return "token";
  if (["pool", "pair", "liquidity_pool"].includes(normalized)) return "pool";
  if (normalized === "wallet") return "wallet";
  if (normalized === "program") return "program";
  if (["transaction", "tx"].includes(normalized)) return "transaction";
  if (source === "pump") return "token";
  if (source === "raydium") return "pool";
  return "unknown";
};

const normalizeTimestamp = (...values: unknown[]): string | undefined => {
  for (const value of values) {
    if (typeof value !== "string" || !value.trim()) continue;
    const parsed = new Date(value);
    if (!Number.isNaN(parsed.getTime())) return parsed.toISOString();
  }
  return undefined;
};

const normalizeMetadata = (value: unknown): Record<string, unknown> => {
  if (!value || typeof value !== "object" || Array.isArray(value)) return {};
  return { ...(value as Record<string, unknown>) };
};

export function normalizeObservation(input: RawObservation): NormalizeResult {
  const errors: string[] = [];
  const moduleId = firstString(input.module_id, input.source, "unknown_source");
  const source = normalizeSource(moduleId, asString(input.source));
  const target = firstString(input.target, input.target_mint, input.mint, input.address);
  const eventType = firstString(input.event_type, "observed");
  const targetType = normalizeTargetType(asString(input.target_type), source);
  const observedAt = normalizeTimestamp(input.observed_at, input.created_at);

  if (!target) errors.push("target is required");
  if (source === "unknown") errors.push("source could not be classified");
  if (!observedAt) errors.push("observed_at or created_at must be a valid timestamp");

  if (errors.length > 0 || !observedAt) return { ok: false, errors };

  return {
    ok: true,
    errors: [],
    observation: {
      schema_version: "1.0",
      source,
      module_id: moduleId,
      event_type: eventType,
      signature: firstString(input.signature) || undefined,
      target,
      target_type: targetType,
      network: firstString(input.network, "solana-mainnet"),
      observed_at: observedAt,
      metadata: normalizeMetadata(input.metadata)
    }
  };
}

export function normalizeBatch(inputs: RawObservation[]): {
  observations: NormalizedObservation[];
  rejected: Array<{ index: number; errors: string[] }>;
} {
  const observations: NormalizedObservation[] = [];
  const rejected: Array<{ index: number; errors: string[] }> = [];

  inputs.forEach((input, index) => {
    const result = normalizeObservation(input);
    if (result.ok && result.observation) observations.push(result.observation);
    else rejected.push({ index, errors: result.errors });
  });

  return { observations, rejected };
}
