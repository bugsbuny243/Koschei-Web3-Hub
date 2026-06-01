import { NextResponse } from "next/server";
import { proxyAuthRequest } from "@/lib/server/auth-api/proxy";

export async function POST(request: Request) {
  const response = await proxyAuthRequest(request, "/auth/logout");
  const redirect = NextResponse.redirect(new URL("/login", request.url), 303);
  const setCookie = response.headers.get("set-cookie");
  if (setCookie) redirect.headers.set("set-cookie", setCookie);
  return redirect;
}
