import "server-only";

type AuthApiError = { ok: false; message: string };

function authApiUrl(path: string) {
  const baseUrl = process.env.AUTH_API_URL?.trim().replace(/\/$/, "");
  return baseUrl ? `${baseUrl}${path}` : null;
}

function jsonResponse(body: unknown, status: number, sourceHeaders?: Headers) {
  const headers = new Headers(sourceHeaders);
  headers.set("Content-Type", "application/json");
  headers.delete("Content-Encoding");
  headers.delete("Content-Length");
  headers.delete("Transfer-Encoding");
  return new Response(JSON.stringify(body), { status, headers });
}

function proxyError(message: string, status = 502) {
  return jsonResponse({ ok: false, message } satisfies AuthApiError, status);
}

export async function proxyMemberAuth(request: Request, path: string) {
  const url = authApiUrl(path);
  if (!url) return proxyError("Auth API is not configured.", 503);

  const headers = new Headers();
  for (const name of ["accept", "content-type", "cookie"]) {
    const value = request.headers.get(name);
    if (value) headers.set(name, value);
  }

  let response: Response;
  try {
    response = await fetch(url, {
      method: request.method,
      headers,
      body: request.method === "GET" || request.method === "HEAD" ? undefined : await request.arrayBuffer(),
      cache: "no-store",
      redirect: "manual",
    });
  } catch {
    return proxyError("Auth API request failed.");
  }

  let text: string;
  try {
    text = await response.text();
  } catch {
    return proxyError("Auth API request failed.");
  }
  if (!text.trim()) return proxyError("Auth API returned an empty response.");

  let body: unknown;
  try {
    body = JSON.parse(text);
  } catch {
    return proxyError("Auth API returned a non-JSON response.");
  }

  return jsonResponse(body, response.status, response.headers);
}

export type MemberSession = { loggedIn: true; sub: string; email: string };

export async function getMemberSession(cookieHeader: string): Promise<MemberSession | null> {
  const url = authApiUrl("/auth/me");
  if (!url) throw new Error("Auth API is not configured.");

  let response: Response;
  try {
    response = await fetch(url, { headers: { cookie: cookieHeader }, cache: "no-store" });
  } catch {
    throw new Error("Auth API request failed.");
  }
  if (response.status === 401) return null;
  if (!response.ok) throw new Error("Auth API request failed.");

  const body = await response.json() as Partial<MemberSession>;
  if (body.loggedIn !== true || typeof body.sub !== "string" || !body.sub || typeof body.email !== "string" || !body.email) {
    throw new Error("Auth API returned an invalid member session.");
  }
  return { loggedIn: true, sub: body.sub, email: body.email };
}
