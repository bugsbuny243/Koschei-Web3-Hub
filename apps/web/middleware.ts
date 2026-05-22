import { NextRequest, NextResponse } from "next/server";
import { verifyToken } from "@/lib/auth";

const OWNER_PREFIX = "/owner";
const DASHBOARD_PREFIX = "/dashboard";

export function middleware(request: NextRequest) {
  const { pathname } = request.nextUrl;
  if (!pathname.startsWith(OWNER_PREFIX) && !pathname.startsWith(DASHBOARD_PREFIX)) {
    return NextResponse.next();
  }

  const token = request.cookies.get("koschei_token")?.value ?? request.headers.get("authorization")?.replace("Bearer ", "");
  if (!token) {
    return NextResponse.redirect(new URL("/auth", request.url));
  }

  const payload = verifyToken(token);
  if (!payload) return NextResponse.redirect(new URL("/auth", request.url));

  if (pathname.startsWith(OWNER_PREFIX) && payload.role !== "owner" && payload.role !== "admin") {
    return NextResponse.redirect(new URL("/dashboard", request.url));
  }

  return NextResponse.next();
}

export const config = {
  matcher: ["/dashboard/:path*", "/owner/:path*"],
};
