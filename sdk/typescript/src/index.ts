export type RiskLevel = "low" | "medium" | "high" | "critical" | string;

export interface SignedVerdict {
  target?: string;
  network?: string;
  grade?: string;
  risk_index?: number;
  risk_level?: RiskLevel;
  verdict?: string;
  recommendation?: string;
  evidence?: string[];
  rule_version?: string;
  signed?: boolean;
  signature?: string;
  created_at?: string;
  [key: string]: unknown;
}

export interface VerdictValidationResult {
  ok: boolean;
  errors: string[];
  verdict?: SignedVerdict;
}

export interface RadarCheckRequest {
  target: string;
  network?: string;
  mode?: string;
}

export interface TokenScanRequest {
  mint: string;
  network?: string;
  include_ai?: boolean;
}

export interface ShieldPreflightRequest {
  target?: string;
  target_mint?: string;
  address?: string;
  transaction?: string;
  wallet?: string;
  network?: string;
  context?: Record<string, unknown>;
}

export interface ArvisClientOptions {
  baseUrl?: string;
  apiKey?: string;
  bearerToken?: string;
  fetchImpl?: typeof fetch;
}

export class ArvisApiError extends Error {
  readonly status: number;
  readonly payload: unknown;

  constructor(status: number, message: string, payload: unknown) {
    super(message);
    this.name = "ArvisApiError";
    this.status = status;
    this.payload = payload;
  }
}

export class ArvisClient {
  private readonly baseUrl: string;
  private readonly apiKey?: string;
  private readonly bearerToken?: string;
  private readonly fetchImpl: typeof fetch;

  constructor(options: ArvisClientOptions = {}) {
    this.baseUrl = (options.baseUrl ?? "https://tradepigloball.co").replace(/\/$/, "");
    this.apiKey = options.apiKey;
    this.bearerToken = options.bearerToken;
    this.fetchImpl = options.fetchImpl ?? fetch;
  }

  async radarCheck<T = { final_verdict?: SignedVerdict }>(request: RadarCheckRequest): Promise<T> {
    return this.request<T>("/api/v1/radar/check", {
      method: "POST",
      auth: "session",
      body: {
        target: request.target,
        network: request.network ?? "solana-mainnet",
        mode: request.mode ?? "sdk"
      }
    });
  }

  async radarFeed<T = Record<string, unknown>>(): Promise<T> {
    return this.request<T>("/api/v1/radar/feed", { method: "GET", auth: "session" });
  }

  async tokenScan<T = { request_id: string; status: string; cost_credits: number }>(request: TokenScanRequest): Promise<T> {
    return this.request<T>("/api/v1/scan/token", {
      method: "POST",
      auth: "apiKey",
      body: {
        mint: request.mint,
        network: request.network ?? "solana-mainnet",
        include_ai: request.include_ai ?? false
      }
    });
  }

  async shieldPreflight<T = SignedVerdict & { action?: string; request_id?: string }>(request: ShieldPreflightRequest): Promise<T> {
    return this.request<T>("/api/v1/shield/preflight", {
      method: "POST",
      auth: "apiKey",
      body: {
        ...request,
        network: request.network ?? "solana-mainnet"
      }
    });
  }

  async usage<T = { usage?: unknown[] }>(): Promise<T> {
    return this.request<T>("/api/v1/usage", { method: "GET", auth: "apiKey" });
  }

  private async request<T>(
    path: string,
    input: { method: "GET" | "POST"; auth: "session" | "apiKey"; body?: unknown }
  ): Promise<T> {
    const headers = new Headers({ Accept: "application/json" });

    if (input.body !== undefined) {
      headers.set("Content-Type", "application/json");
    }

    if (input.auth === "apiKey") {
      if (!this.apiKey) {
        throw new Error("ARVIS API key is required for this endpoint.");
      }
      headers.set("X-API-Key", this.apiKey);
    } else {
      if (!this.bearerToken) {
        throw new Error("ARVIS bearer token is required for this endpoint.");
      }
      headers.set("Authorization", `Bearer ${this.bearerToken}`);
    }

    const response = await this.fetchImpl(`${this.baseUrl}${path}`, {
      method: input.method,
      headers,
      body: input.body === undefined ? undefined : JSON.stringify(input.body)
    });

    let payload: unknown = null;
    try {
      payload = await response.json();
    } catch {
      payload = null;
    }

    if (!response.ok) {
      const message = this.errorMessage(payload) ?? `ARVIS request failed with status ${response.status}`;
      throw new ArvisApiError(response.status, message, payload);
    }

    return payload as T;
  }

  private errorMessage(payload: unknown): string | undefined {
    if (!payload || typeof payload !== "object") return undefined;
    const data = payload as Record<string, unknown>;
    for (const key of ["message", "error", "code"]) {
      const value = data[key];
      if (typeof value === "string" && value.trim()) return value;
    }
    return undefined;
  }
}

export function validateSignedVerdict(value: unknown): VerdictValidationResult {
  if (!value || typeof value !== "object" || Array.isArray(value)) {
    return { ok: false, errors: ["verdict must be an object"] };
  }

  const verdict = value as SignedVerdict;
  const errors: string[] = [];

  if (verdict.signed !== true) errors.push("signed must be true");
  if (typeof verdict.risk_index !== "number" || !Number.isFinite(verdict.risk_index)) {
    errors.push("risk_index must be a finite number");
  } else if (!Number.isInteger(verdict.risk_index)) {
    errors.push("risk_index must be an integer");
  } else if (verdict.risk_index < 0 || verdict.risk_index > 100) {
    errors.push("risk_index must be between 0 and 100");
  }
  if (typeof verdict.grade !== "string" || !/^[A-F]$/.test(verdict.grade)) {
    errors.push("grade must be A through F");
  }
  if (
    typeof verdict.risk_level !== "string" ||
    !["low", "medium", "high", "critical"].includes(verdict.risk_level)
  ) {
    errors.push("risk_level must be low, medium, high, or critical");
  }
  if (
    !Array.isArray(verdict.evidence) ||
    verdict.evidence.length === 0 ||
    verdict.evidence.some((item) => typeof item !== "string" || item.trim() === "")
  ) {
    errors.push("evidence must be a non-empty array of strings");
  }
  if (typeof verdict.rule_version !== "string" || verdict.rule_version.trim() === "") {
    errors.push("rule_version is required");
  }

  return errors.length === 0
    ? { ok: true, errors: [], verdict }
    : { ok: false, errors };
}

export function isSignedVerdict(value: unknown): value is SignedVerdict {
  return validateSignedVerdict(value).ok;
}
