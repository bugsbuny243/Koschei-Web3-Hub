declare module "pg" {
  export type PoolConfig = {
    connectionString?: string;
    connectionTimeoutMillis?: number;
    idleTimeoutMillis?: number;
    allowExitOnIdle?: boolean;
    max?: number;
    ssl?: boolean | Record<string, unknown>;
  };

  export class Pool {
    constructor(config?: PoolConfig);
    query<T = any>(text: string, params?: unknown[]): Promise<{ rows: T[] }>;
  }
}
