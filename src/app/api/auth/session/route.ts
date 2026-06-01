import { NextResponse } from "next/server";
import { getUserSession } from "@/lib/server/user-auth";

export async function GET() {
  try {
    const session = await getUserSession();
    return NextResponse.json({ loggedIn: Boolean(session), email: session?.email });
  } catch {
    return NextResponse.json({ error: "Member sessions are unavailable. Configure USER_SESSION_SECRET or MEMBER_SESSION_SECRET." }, { status: 503 });
  }
}
