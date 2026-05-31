import { NextResponse } from "next/server";
import { upsertUserProfile } from "@/lib/server/db";
import { appendNeonAuthCookies, authenticateWithNeonAuth } from "@/lib/server/neon-auth";
import { isValidEmail, isValidPassword, normalizeEmail } from "@/lib/server/user-auth";

export async function POST(request: Request) {
  let body: Record<string, unknown>;
  try { body = await request.json() as Record<string, unknown>; } catch { return NextResponse.json({ error: "Invalid JSON body." }, { status: 400 }); }
  const email = normalizeEmail(body.email);
  if (!isValidEmail(email) || !isValidPassword(body.password)) return NextResponse.json({ error: "Enter a valid email and a password with at least 8 characters." }, { status: 400 });
  try {
    const auth = await authenticateWithNeonAuth("signup", email, body.password);
    await upsertUserProfile(auth.user.id, auth.user.email);
    return appendNeonAuthCookies(NextResponse.json({ email: auth.user.email }, { status: 201 }), auth.cookies);
  } catch {
    return NextResponse.json({ error: "Could not create account. The email may already be registered." }, { status: 400 });
  }
}
