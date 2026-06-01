import { NextResponse } from "next/server";
import { getUserProfileForLogin } from "@/lib/server/db";
import { isValidEmail, isValidPassword, normalizeEmail, setUserCookie, verifyPassword } from "@/lib/server/user-auth";

export async function POST(request: Request) {
  let body: Record<string, unknown>;
  try { body = await request.json() as Record<string, unknown>; } catch { return NextResponse.json({ error: "Invalid JSON body." }, { status: 400 }); }
  const email = normalizeEmail(body.email);
  if (!isValidEmail(email) || !isValidPassword(body.password)) return NextResponse.json({ error: "Invalid email or password." }, { status: 401 });
  try {
    const profile = await getUserProfileForLogin(email);
    if (!profile?.password_hash || !verifyPassword(body.password, profile.password_hash)) return NextResponse.json({ error: "Invalid email or password." }, { status: 401 });
    await setUserCookie(profile.email.toLowerCase());
    return NextResponse.json({ email: profile.email });
  } catch {
    return NextResponse.json({ error: "Could not sign in." }, { status: 503 });
  }
}
