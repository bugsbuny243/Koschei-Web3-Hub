import { NextResponse } from "next/server";
import { createMemberAccount } from "@/lib/server/db";
import { assertMemberSessionConfigured, hashPassword, isValidEmail, isValidPassword, normalizeEmail, setUserCookie } from "@/lib/server/user-auth";

const ACCOUNT_EXISTS = "Account already exists. Please sign in.";

export async function POST(request: Request) {
  let body: Record<string, unknown>;
  try { body = await request.json() as Record<string, unknown>; } catch { return NextResponse.json({ error: "Invalid JSON body." }, { status: 400 }); }
  const email = normalizeEmail(body.email);
  if (!isValidEmail(email) || !isValidPassword(body.password)) return NextResponse.json({ error: "Enter a valid email and a password with at least 8 characters." }, { status: 400 });
  try {
    assertMemberSessionConfigured();
    const account = await createMemberAccount(email, hashPassword(body.password));
    await setUserCookie(account.email);
    return NextResponse.json({ email: account.email }, { status: 201 });
  } catch (reason) {
    const message = reason instanceof Error ? reason.message : "";
    if (message === ACCOUNT_EXISTS) return NextResponse.json({ error: ACCOUNT_EXISTS }, { status: 409 });
    if (message.includes("SESSION_SECRET")) return NextResponse.json({ error: "Could not create account. Configure USER_SESSION_SECRET or MEMBER_SESSION_SECRET." }, { status: 503 });
    return NextResponse.json({ error: "Could not create account. Confirm member auth migration was applied." }, { status: 503 });
  }
}
