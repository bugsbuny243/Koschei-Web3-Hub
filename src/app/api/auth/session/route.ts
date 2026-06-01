import { NextResponse } from "next/server";
import { getNeonAuthSession } from "@/lib/server/neon-auth";

export async function GET() {
  const user = await getNeonAuthSession();
  if (!user) return NextResponse.json({ error: "Not authenticated." }, { status: 401 });
  return NextResponse.json({ email: user.email, id: user.id });
}
