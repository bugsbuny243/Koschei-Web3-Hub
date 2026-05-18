declare module 'pg' {
  export class Pool {
    constructor(config: { connectionString: string });
    query<T = any>(text: string, params?: unknown[]): Promise<{ rows: T[] }>;
  }
}
