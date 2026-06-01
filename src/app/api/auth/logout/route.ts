import { NextResponse } from "next/server";
import { signOutFromNeonAuth, appendNeonAuthCookies } from "@/lib/server/neon-auth";

export async function POST(request: Request) {
  const authCookies = await signOutFromNeonAuth();
  const response = NextResponse.redirect(new URL("/login", request.url), 303);
  return appendNeonAuthCookies(response, authCookies);
}
