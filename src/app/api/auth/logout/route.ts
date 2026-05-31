import { NextResponse } from "next/server";
import { appendNeonAuthCookies, signOutFromNeonAuth } from "@/lib/server/neon-auth";

export async function POST(request: Request) {
  const response = NextResponse.redirect(new URL("/login", request.url), 303);
  return appendNeonAuthCookies(response, await signOutFromNeonAuth());
}
