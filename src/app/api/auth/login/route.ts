import { NextResponse } from "next/server";
import { authenticateWithNeonAuth } from "@/lib/server/neon-auth";
import { isValidEmail, isValidPassword, normalizeEmail, setUserCookie } from "@/lib/server/user-auth";

export async function POST(request: Request) {
  let body: Record<string, unknown>;
  try { body = await request.json() as Record<string, unknown>; } catch { return NextResponse.json({ error: "Invalid JSON body." }, { status: 400 }); }
  const email = normalizeEmail(body.email);
  if (!isValidEmail(email) || !isValidPassword(body.password)) return NextResponse.json({ error: "Invalid email or password." }, { status: 401 });
  try {
    const { user } = await authenticateWithNeonAuth("login", email, body.password as string);
    await setUserCookie(user.email);
    return NextResponse.json({ email: user.email });
  } catch {
    return NextResponse.json({ error: "Invalid email or password." }, { status: 401 });
  }
}
