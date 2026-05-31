import { NextResponse } from "next/server";
import { isAdminRequest } from "@/lib/server/admin-auth";
import { listAdminOrders } from "@/lib/server/db";
export async function GET() { if (!await isAdminRequest()) return NextResponse.json({ error: "Unauthorized." }, { status: 401 }); try { return NextResponse.json({ orders: await listAdminOrders() }); } catch { return NextResponse.json({ error: "Admin orders are unavailable." }, { status: 503 }); } }
