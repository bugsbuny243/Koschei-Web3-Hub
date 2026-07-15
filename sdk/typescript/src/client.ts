import type { SignedVerdict } from "./index.js";

export interface RadarCheckRequest {
  target: string;
  network?: string;
  mode?: string;
}

export interface TokenScanRequest {
  mint: string;
  network?: string;
  include_ai?: boolean;
  idempotencyKey?: string;
}

export interface TokenBatchScanRequest {
  mints: string[];
  network?: string;
  include_ai?: boolean;
  idempotencyKey?: string;
}

export interface QueuedScanResponse {
  request_id: string;
  status: string;
  mode?: "single" | "batch";
  target_count?: number;
  cost_credits: number;
  result_url?: string;
  idempotent_replay?: boolean;
}

export interface AsyncRequestStatus<T = unknown> {
  ok: boolean;
  request_id: string;
  endpoint: string;
  status: string;
  terminal: boolean;
  credits_reserved: number;
  credits_charged: number;
  error_code?: string;
  latency_ms?: number | null;
  idempotency_key?: string;
  result_available: boolean;
  result: T;
  poll_after_ms?: number;
  created_at: string;
  completed_at?: string | null;
}

export interface WaitForRequestOptions {
  timeoutMs?: number;
  pollIntervalMs?: number;
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

  async tokenScan<T = QueuedScanResponse>(request: TokenScanRequest): Promise<T> {
    return this.request<T>("/api/v1/scan/token", {
      method: "POST",
      auth: "apiKey",
      headers: idempotencyHeaders(request.idempotencyKey),
      body: {
        mint: request.mint,
        network: request.network ?? "solana-mainnet",
        include_ai: request.include_ai ?? false
      }
    });
  }

  async tokenScanBatch<T = QueuedScanResponse>(request: TokenBatchScanRequest): Promise<T> {
    const mints = [...new Set(request.mints.map((mint) => mint.trim()).filter(Boolean))];
    if (mints.length === 0) throw new Error("At least one token mint is required.");
    if (mints.length > 20) throw new Error("A batch can contain at most 20 token mints.");
    return this.request<T>("/api/v1/scan/token", {
      method: "POST",
      auth: "apiKey",
      headers: idempotencyHeaders(request.idempotencyKey),
      body: {
        mints,
        network: request.network ?? "solana-mainnet",
        include_ai: request.include_ai ?? false
      }
    });
  }

  async requestStatus<T = unknown>(requestId: string): Promise<AsyncRequestStatus<T>> {
    const normalized = requestId.trim();
    if (!normalized) throw new Error("requestId is required.");
    return this.request<AsyncRequestStatus<T>>(`/api/v1/usage?request_id=${encodeURIComponent(normalized)}`, {
      method: "GET",
      auth: "apiKey"
    });
  }

  async waitForRequest<T = unknown>(requestId: string, options: WaitForRequestOptions = {}): Promise<AsyncRequestStatus<T>> {
    const timeoutMs = Math.max(1_000, options.timeoutMs ?? 120_000);
    const pollIntervalMs = Math.max(250, options.pollIntervalMs ?? 1_500);
    const startedAt = Date.now();

    for (;;) {
      const status = await this.requestStatus<T>(requestId);
      if (status.terminal) return status;
      if (Date.now() - startedAt >= timeoutMs) {
        throw new Error(`Timed out waiting for ARVIS request ${requestId}.`);
      }
      await delay(Math.max(250, status.poll_after_ms ?? pollIntervalMs));
    }
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

  async usage<T = { usage?: unknown[] }>(options: { includeResults?: boolean } = {}): Promise<T> {
    const query = options.includeResults ? "?include_results=1" : "";
    return this.request<T>(`/api/v1/usage${query}`, { method: "GET", auth: "apiKey" });
  }

  private async request<T>(
    path: string,
    input: {
      method: "GET" | "POST";
      auth: "session" | "apiKey";
      body?: unknown;
      headers?: Record<string, string>;
    }
  ): Promise<T> {
    const headers = new Headers({ Accept: "application/json", ...input.headers });

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

function idempotencyHeaders(value?: string): Record<string, string> | undefined {
  const normalized = value?.trim();
  if (!normalized) return undefined;
  if (normalized.length > 128) throw new Error("Idempotency key must be 128 characters or fewer.");
  return { "Idempotency-Key": normalized };
}

function delay(ms: number): Promise<void> {
  return new Promise((resolve) => setTimeout(resolve, ms));
}
