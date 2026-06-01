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
