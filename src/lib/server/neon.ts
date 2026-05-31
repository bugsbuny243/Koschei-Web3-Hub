import "server-only";

type QueryValue = string | number | boolean | null | Record<string, unknown> | unknown[];
type NeonResult = { fields: { name: string }[]; rows: unknown[][] };

function databaseUrl() {
  const value = process.env.DATABASE_URL?.trim();
  if (!value) throw new Error("DATABASE_URL is not configured.");
  return value;
}

export async function query<T extends Record<string, unknown>>(sql: string, params: QueryValue[] = []): Promise<T[]> {
  const connectionString = databaseUrl();
  const url = new URL(connectionString);
  const response = await fetch(`https://${url.hostname}/sql`, {
    method: "POST",
    headers: { "Content-Type": "application/json", "Neon-Connection-String": connectionString, "Neon-Raw-Text-Output": "true", "Neon-Array-Mode": "true" },
    body: JSON.stringify({ query: sql, params }),
    cache: "no-store",
  });
  if (!response.ok) throw new Error(`Neon query failed (${response.status}).`);
  const result = await response.json() as NeonResult;
  return result.rows.map((row) => Object.fromEntries(result.fields.map((field, index) => [field.name, row[index]])) as T);
}
