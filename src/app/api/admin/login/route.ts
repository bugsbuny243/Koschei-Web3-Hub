import { NextResponse } from "next/server";
import { isValidAdminCredentials, setAdminCookie } from "@/lib/server/admin-auth";
export async function POST(request: Request) { let body: unknown; try { body = await request.json(); } catch { return NextResponse.json({ ok: false }, { status: 400 }); } const { email, password } = body as Record<string, unknown>; if (!isValidAdminCredentials(email, password)) return NextResponse.json({ ok: false }, { status: 401 }); await setAdminCookie(); return NextResponse.json({ ok: true }); }
