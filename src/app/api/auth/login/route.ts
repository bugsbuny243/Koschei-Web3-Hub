import { NextResponse } from "next/server";
import { upsertUserProfile } from "@/lib/server/db";
import { authenticateWithNeonAuth, NeonAuthConfigurationError } from "@/lib/server/neon-auth";
import { assertMemberSessionConfigured, isValidEmail, isValidPassword, MemberSessionConfigurationError, normalizeEmail, setUserCookie } from "@/lib/server/user-auth";

export async function POST(request: Request) {
  let body: Record<string, unknown>;
  try { body = await request.json() as Record<string, unknown>; } catch { return NextResponse.json({ error: "Invalid JSON body." }, { status: 400 }); }
  const email = normalizeEmail(body.email);
  if (!isValidEmail(email) || !isValidPassword(body.password)) return NextResponse.json({ error: "Invalid email or password." }, { status: 401 });
  try {
    assertMemberSessionConfigured();
    const identity = await authenticateWithNeonAuth("login", email, body.password as string);
    await upsertUserProfile(identity.sub, identity.email);
    await setUserCookie(identity.sub, identity.email);
    return NextResponse.json({ email: identity.email });
  } catch (error) {
    if (error instanceof MemberSessionConfigurationError) return NextResponse.json({ error: error.message }, { status: 503 });
    if (error instanceof NeonAuthConfigurationError) return NextResponse.json({ error: "Auth service is not configured." }, { status: 503 });
    return NextResponse.json({ error: "Invalid email or password." }, { status: 401 });
  }
}
