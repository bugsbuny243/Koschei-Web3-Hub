import { NextResponse } from "next/server";
import { upsertUserProfile } from "@/lib/server/db";
import { appendNeonAuthCookies, authenticateWithNeonAuth } from "@/lib/server/neon-auth";
import { isValidEmail, isValidPassword, normalizeEmail } from "@/lib/server/user-auth";

export async function POST(request: Request) {
  let body: Record<string, unknown>;
  try { body = await request.json() as Record<string, unknown>; } catch { return NextResponse.json({ error: "Invalid JSON body." }, { status: 400 }); }
  const email = normalizeEmail(body.email);
  if (!isValidEmail(email) || !isValidPassword(body.password)) return NextResponse.json({ error: "Invalid email or password." }, { status: 401 });
  try {
    const auth = await authenticateWithNeonAuth("login", email, body.password);
    await upsertUserProfile(auth.user.id, auth.user.email);
    return appendNeonAuthCookies(NextResponse.json({ email: auth.user.email }), auth.cookies);
  } catch {
    return NextResponse.json({ error: "Invalid email or password." }, { status: 401 });
  }
}
