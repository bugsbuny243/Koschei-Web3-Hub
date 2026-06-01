import { proxyMemberAuth } from "@/lib/server/auth-api";

export async function POST(request: Request) {
  const response = await proxyMemberAuth(request, "/auth/logout");
  const headers = new Headers(response.headers);
  headers.set("Location", new URL("/login", request.url).toString());
  return new Response(null, { status: 303, headers });
}
