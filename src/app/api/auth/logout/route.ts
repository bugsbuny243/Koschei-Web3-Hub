import { NextResponse } from "next/server";
import { clearUserCookie } from "@/lib/server/user-auth";

export async function POST(request: Request) {
  await clearUserCookie();
  return NextResponse.redirect(new URL("/login", request.url), 303);
}
