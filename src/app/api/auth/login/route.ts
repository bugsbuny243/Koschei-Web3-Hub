import { NextResponse } from "next/server";
import { upsertUserProfile } from "@/lib/server/db";
import { authenticateWithNeonAuth, isCredentialRejection, NeonAuthConfigurationError, NeonAuthProviderError, NeonAuthRequestError, NeonAuthSessionError, NeonAuthVerificationError } from "@/lib/server/neon-auth";
import { assertMemberSessionConfigured, isValidEmail, isValidPassword, MemberSessionConfigurationError, normalizeEmail, setUserCookie } from "@/lib/server/user-auth";

function logLoginIssue(event: string, error: unknown) {
  console.error(`[member-login] ${event}`, { error: error instanceof Error ? error.message : "Unknown login failure." });
}

export async function POST(request: Request) {
  let body: Record<string, unknown>;
  try { body = await request.json() as Record<string, unknown>; } catch { return NextResponse.json({ error: "Invalid JSON body." }, { status: 400 }); }
  const email = normalizeEmail(body.email);
  if (!isValidEmail(email) || !isValidPassword(body.password)) return NextResponse.json({ error: "Invalid email or password." }, { status: 401 });
  try {
    assertMemberSessionConfigured();
    const identity = await authenticateWithNeonAuth("login", email, body.password as string);
    try { await upsertUserProfile(identity.sub, identity.email); } catch (error) {
      logLoginIssue("profile upsert failed", error);
      return NextResponse.json({ error: "Could not create user profile." }, { status: 503 });
    }
    await setUserCookie(identity.sub, identity.email);
    return NextResponse.json({ email: identity.email });
  } catch (error) {
    if (error instanceof MemberSessionConfigurationError || error instanceof NeonAuthConfigurationError) return NextResponse.json({ error: "Auth service is not configured." }, { status: 503 });
    if (error instanceof NeonAuthRequestError && isCredentialRejection(error)) return NextResponse.json({ error: "Invalid email or password." }, { status: 401 });
    if (error instanceof NeonAuthSessionError) return NextResponse.json({ error: "Auth provider did not return a session." }, { status: 502 });
    if (error instanceof NeonAuthVerificationError) return NextResponse.json({ error: "Auth token verification failed." }, { status: 502 });
    if (!(error instanceof NeonAuthProviderError || error instanceof NeonAuthRequestError)) logLoginIssue("unexpected auth failure", error);
    return NextResponse.json({ error: "Auth provider request failed." }, { status: 502 });
  }
}
