import { createHmac, timingSafeEqual } from "node:crypto";
import { NextRequest, NextResponse } from "next/server";

const COOKIE_NAME = "koschei_admin";

function hasValidAdminCookie(request: NextRequest) {
  if (!process.env.ADMIN_EMAIL || !process.env.ADMIN_PASSWORD) return false;
  const value = request.cookies.get(COOKIE_NAME)?.value;
  if (!value) return false;
  const expected = createHmac("sha256", process.env.ADMIN_PASSWORD).update(process.env.ADMIN_EMAIL).digest("hex");
  return value.length === expected.length && timingSafeEqual(Buffer.from(value), Buffer.from(expected));
}

export function proxy(request: NextRequest) {
  if (request.nextUrl.pathname === "/api/admin/login" || hasValidAdminCookie(request)) return NextResponse.next();
  return NextResponse.json({ error: "Unauthorized." }, { status: 401 });
}

export const config = { matcher: "/api/admin/:path*" };
