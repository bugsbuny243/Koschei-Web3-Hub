import { NextResponse } from "next/server";
import { upsertUserProfile } from "@/lib/server/db";
import { authenticateWithNeonAuth, NeonAuthConfigurationError, NeonAuthProviderRequestError, NeonAuthRequestError, NeonAuthSessionError } from "@/lib/server/neon-auth";
import { assertMemberSessionConfigured, isValidEmail, isValidPassword, MemberSessionConfigurationError, normalizeEmail, setUserCookie } from "@/lib/server/user-auth";

function logSignupError(category: string, error: unknown) {
  console.error("[auth/signup] signup failed", {
    category,
    errorType: error instanceof Error ? error.constructor.name : typeof error,
    providerStatus: error instanceof NeonAuthRequestError ? error.status : undefined,
  });
}

function isDuplicateAccountError(error: unknown) {
  return error instanceof NeonAuthRequestError
    && (error.status === 409 || /duplicate|already[\s_-]*exists?/i.test(error.message));
}

export async function POST(request: Request) {
  let body: Record<string, unknown>;
  try { body = await request.json() as Record<string, unknown>; } catch { return NextResponse.json({ error: "Invalid JSON body." }, { status: 400 }); }
  const email = normalizeEmail(body.email);
  if (!isValidEmail(email) || !isValidPassword(body.password)) return NextResponse.json({ error: "Enter a valid email and a password with at least 8 characters." }, { status: 400 });

  let identity: Awaited<ReturnType<typeof authenticateWithNeonAuth>>;
  try {
    assertMemberSessionConfigured();
    identity = await authenticateWithNeonAuth("signup", email, body.password);
  } catch (error) {
    if (error instanceof MemberSessionConfigurationError || error instanceof NeonAuthConfigurationError) {
      logSignupError("configuration", error);
      return NextResponse.json({ error: "Auth service is not configured." }, { status: 503 });
    }
    if (isDuplicateAccountError(error)) {
      logSignupError("duplicate-account", error);
      return NextResponse.json({ error: "Account already exists. Please sign in." }, { status: 409 });
    }
    if (error instanceof NeonAuthSessionError) {
      logSignupError("missing-session", error);
      return NextResponse.json({ error: "Auth provider did not return a session." }, { status: 502 });
    }
    if (error instanceof NeonAuthProviderRequestError || error instanceof NeonAuthRequestError || error instanceof Error) {
      logSignupError("provider-request", error);
      return NextResponse.json({ error: "Auth provider request failed." }, { status: 502 });
    }
    logSignupError("provider-request", error);
    return NextResponse.json({ error: "Auth provider request failed." }, { status: 502 });
  }

  try {
    await upsertUserProfile(identity.sub, identity.email);
  } catch (error) {
    logSignupError("profile-upsert", error);
    return NextResponse.json({ error: "Could not create user profile." }, { status: 500 });
  }

  try {
    await setUserCookie(identity.sub, identity.email);
  } catch (error) {
    logSignupError("session-cookie", error);
    return NextResponse.json({ error: "Auth provider did not return a session." }, { status: 500 });
  }
  return NextResponse.json({ email: identity.email }, { status: 201 });
}
