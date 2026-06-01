import { NextResponse } from "next/server";
import { getUserSession } from "@/lib/server/user-auth";

export async function GET() {
  try {
    const session = await getUserSession();
    if (!session) return NextResponse.json({ loggedIn: false }, { status: 401 });
    return NextResponse.json({ loggedIn: true, email: session.email });
  } catch {
    return NextResponse.json({ loggedIn: false }, { status: 503 });
  }
}
