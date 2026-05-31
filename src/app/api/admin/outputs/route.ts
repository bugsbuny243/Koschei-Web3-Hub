import { NextResponse } from "next/server";
import { isAdminRequest } from "@/lib/server/admin-auth";
import { listAdminOutputs } from "@/lib/server/db";
export async function GET() { if (!await isAdminRequest()) return NextResponse.json({ error: "Unauthorized." }, { status: 401 }); try { return NextResponse.json({ outputs: await listAdminOutputs() }); } catch { return NextResponse.json({ error: "Admin outputs are unavailable." }, { status: 503 }); } }
