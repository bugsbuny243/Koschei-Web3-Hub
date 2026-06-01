import { NextResponse } from "next/server";
import { getMemberAccountForLogin } from "@/lib/server/db";
import { assertMemberSessionConfigured, isValidEmail, isValidPassword, normalizeEmail, setUserCookie, verifyPassword } from "@/lib/server/user-auth";

export async function POST(request: Request) {
  let body: Record<string, unknown>;
  try { body = await request.json() as Record<string, unknown>; } catch { return NextResponse.json({ error: "Invalid JSON body." }, { status: 400 }); }
  const email = normalizeEmail(body.email);
  if (!isValidEmail(email) || !isValidPassword(body.password)) return NextResponse.json({ error: "Invalid email or password." }, { status: 401 });
  try {
    assertMemberSessionConfigured();
    const account = await getMemberAccountForLogin(email);
    if (!account || !verifyPassword(body.password, account.password_hash)) return NextResponse.json({ error: "Invalid email or password." }, { status: 401 });
    await setUserCookie(account.email);
    return NextResponse.json({ email: account.email });
  } catch (reason) {
    const message = reason instanceof Error ? reason.message : "";
    if (message.includes("SESSION_SECRET")) return NextResponse.json({ error: "Could not sign in. Configure USER_SESSION_SECRET or MEMBER_SESSION_SECRET." }, { status: 503 });
    return NextResponse.json({ error: "Could not sign in. Confirm member auth migration was applied." }, { status: 503 });
  }
}
