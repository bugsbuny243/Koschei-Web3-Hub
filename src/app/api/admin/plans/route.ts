import { NextResponse } from "next/server";
import { isAdminRequest } from "@/lib/server/admin-auth";
import { listAdminPlans } from "@/lib/server/db";

export async function GET() {
  if (!await isAdminRequest()) return NextResponse.json({ error: "Unauthorized." }, { status: 401 });
  try {
    return NextResponse.json({ plans: await listAdminPlans() });
  } catch {
    return NextResponse.json({ error: "Admin plans are unavailable." }, { status: 503 });
  }
}
