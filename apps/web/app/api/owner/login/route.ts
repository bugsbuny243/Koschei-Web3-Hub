import { NextResponse } from "next/server";
import { OWNER_SESSION_COOKIE, isValidAdminPassword } from "@/lib/owner-auth";

export async function POST(req: Request) {
  const body = await req.json().catch(() => ({}));
  const password = String(body.password ?? "");
  if (!isValidAdminPassword(password)) return NextResponse.json({ error: "unauthorized" }, { status: 401 });

  const response = NextResponse.json({ ok: true });
  response.cookies.set(OWNER_SESSION_COOKIE, "valid", {
    httpOnly: true,
    secure: process.env.NODE_ENV === "production",
    sameSite: "lax",
    path: "/",
    maxAge: 60 * 60 * 8,
  });
  return response;
}
