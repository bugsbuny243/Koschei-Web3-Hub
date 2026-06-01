import { NextResponse } from "next/server";
import { upsertUserProfile } from "@/lib/server/db";
import { authenticateWithNeonAuth, NeonAuthConfigurationError } from "@/lib/server/neon-auth";
import { assertMemberSessionConfigured, isValidEmail, isValidPassword, MemberSessionConfigurationError, normalizeEmail, setUserCookie } from "@/lib/server/user-auth";

export async function POST(request: Request) {
  let body: Record<string, unknown>;
  try { body = await request.json() as Record<string, unknown>; } catch { return NextResponse.json({ error: "Invalid JSON body." }, { status: 400 }); }
  const email = normalizeEmail(body.email);
  if (!isValidEmail(email) || !isValidPassword(body.password)) return NextResponse.json({ error: "Enter a valid email and a password with at least 8 characters." }, { status: 400 });
  try {
    assertMemberSessionConfigured();
    const identity = await authenticateWithNeonAuth("signup", email, body.password as string);
    await upsertUserProfile(identity.sub, identity.email);
    await setUserCookie(identity.sub, identity.email);
    return NextResponse.json({ email: identity.email }, { status: 201 });
  } catch (error) {
    if (error instanceof MemberSessionConfigurationError) return NextResponse.json({ error: error.message }, { status: 503 });
    if (error instanceof NeonAuthConfigurationError) return NextResponse.json({ error: "Auth service is not configured." }, { status: 503 });
    return NextResponse.json({ error: "Account already exists. Please sign in." }, { status: 400 });
  }
}
