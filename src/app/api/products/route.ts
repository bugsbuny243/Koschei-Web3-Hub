import { NextResponse } from "next/server";
import { getProducts } from "@/lib/server/db";
export async function GET() { try { return NextResponse.json({ products: await getProducts() }); } catch { return NextResponse.json({ error: "Products are unavailable." }, { status: 503 }); } }
