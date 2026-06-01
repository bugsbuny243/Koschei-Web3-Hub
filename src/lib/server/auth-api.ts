import "server-only";

function authApiUrl(path: string) {
  const baseUrl = process.env.AUTH_API_URL?.trim().replace(/\/$/, "");
  if (!baseUrl) throw new Error("AUTH_API_URL is not configured.");
  return `${baseUrl}${path}`;
}

export async function proxyMemberAuth(request: Request, path: string) {
  const headers = new Headers();
  for (const name of ["accept", "content-type", "cookie"]) {
    const value = request.headers.get(name);
    if (value) headers.set(name, value);
  }
  const response = await fetch(authApiUrl(path), {
    method: request.method,
    headers,
    body: request.method === "GET" || request.method === "HEAD" ? undefined : await request.arrayBuffer(),
    cache: "no-store",
    redirect: "manual",
  });
  return new Response(response.body, { status: response.status, headers: response.headers });
}
