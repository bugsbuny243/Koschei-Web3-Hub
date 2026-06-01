import "server-only";
import { NextResponse } from "next/server";

function authApiUrl(path: string) {
  const baseUrl = process.env.AUTH_API_URL?.trim().replace(/\/+$/, "");
  if (!baseUrl) throw new Error("AUTH_API_URL is not configured.");
  return `${baseUrl}${path}`;
}

export async function proxyAuthRequest(request: Request, path: string) {
  try {
    const headers = new Headers();
    const contentType = request.headers.get("content-type");
    const cookie = request.headers.get("cookie");
    if (contentType) headers.set("content-type", contentType);
    if (cookie) headers.set("cookie", cookie);
    const response = await fetch(authApiUrl(path), {
      method: request.method,
      headers,
      body: request.method === "GET" || request.method === "HEAD" ? undefined : await request.text(),
      cache: "no-store",
    });
    const proxyResponse = new NextResponse(await response.text(), {
      status: response.status,
      headers: { "content-type": response.headers.get("content-type") || "application/json" },
    });
    const setCookie = response.headers.get("set-cookie");
    if (setCookie) proxyResponse.headers.set("set-cookie", setCookie);
    return proxyResponse;
  } catch (error) {
    console.error("[auth-api-proxy] request failed", { path, error: error instanceof Error ? error.message : "Unknown auth-api failure." });
    return NextResponse.json({ error: "Auth service is unavailable." }, { status: 503 });
  }
}
