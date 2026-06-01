import { NextResponse } from "next/server";
import { getUserSession, MemberSessionConfigurationError } from "@/lib/server/user-auth";

export async function GET() {
  try {
    const session = await getUserSession();
    if (!session) return NextResponse.json({ loggedIn: false }, { status: 401 });
    return NextResponse.json({ loggedIn: true, email: session.email });
  } catch (error) {
    if (error instanceof MemberSessionConfigurationError) return NextResponse.json({ loggedIn: false, error: error.message }, { status: 503 });
    return NextResponse.json({ loggedIn: false }, { status: 503 });
  }
}
