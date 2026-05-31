import { NextResponse } from "next/server";
import { createUserProfile } from "@/lib/server/db";
import { hashPassword, isValidEmail, isValidPassword, normalizeEmail, setUserCookie } from "@/lib/server/user-auth";

export async function POST(request: Request) {
  let body: Record<string, unknown>;
  try { body = await request.json() as Record<string, unknown>; } catch { return NextResponse.json({ error: "Invalid JSON body." }, { status: 400 }); }
  const email = normalizeEmail(body.email);
  if (!isValidEmail(email) || !isValidPassword(body.password)) return NextResponse.json({ error: "Enter a valid email and a password with at least 8 characters." }, { status: 400 });
  try {
    await createUserProfile(email, hashPassword(body.password));
    await setUserCookie(email);
    return NextResponse.json({ email }, { status: 201 });
  } catch {
    return NextResponse.json({ error: "Could not create account. The email may already be registered." }, { status: 400 });
  }
}
