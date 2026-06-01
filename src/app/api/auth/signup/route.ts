import { NextResponse } from "next/server";
import { authenticateWithNeonAuth } from "@/lib/server/neon-auth";
import { isValidEmail, isValidPassword, normalizeEmail, setUserCookie } from "@/lib/server/user-auth";

export async function POST(request: Request) {
  let body: Record<string, unknown>;
  try { body = await request.json() as Record<string, unknown>; } catch { return NextResponse.json({ error: "Invalid JSON body." }, { status: 400 }); }
  const email = normalizeEmail(body.email);
  if (!isValidEmail(email) || !isValidPassword(body.password)) return NextResponse.json({ error: "Enter a valid email and a password with at least 8 characters." }, { status: 400 });
  try {
    const { user } = await authenticateWithNeonAuth("signup", email, body.password as string);
    await setUserCookie(user.email);
    return NextResponse.json({ email: user.email }, { status: 201 });
  } catch {
    return NextResponse.json({ error: "Could not create account. The email may already be registered." }, { status: 400 });
  }
}
