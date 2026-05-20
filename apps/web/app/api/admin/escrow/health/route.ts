import { NextResponse } from "next/server";

const requiredNames = [
  "ESCROW_ENV",
  "ESCROW_API_BASE_URL",
  "ESCROW_EMAIL",
  "ESCROW_API_KEY",
  "ESCROW_DEFAULT_SELLER_EMAIL",
  "ESCROW_DEFAULT_CURRENCY",
  "ESCROW_FEE_PAYER"
] as const;

export async function GET(req: Request) {
  const url = new URL(req.url);
  if (url.searchParams.get("password") !== process.env.ADMIN_PASSWORD) {
    return NextResponse.json({ error: "unauthorized" }, { status: 401 });
  }

  const missing = requiredNames.filter((name) => !process.env[name]);
  if (missing.length) {
    return NextResponse.json({ ok: false, missing }, { status: 500 });
  }

  return NextResponse.json({
    ok: true,
    env: process.env.ESCROW_ENV,
    baseUrlConfigured: Boolean(process.env.ESCROW_API_BASE_URL),
    emailConfigured: Boolean(process.env.ESCROW_EMAIL),
    apiKeyConfigured: Boolean(process.env.ESCROW_API_KEY),
    sellerEmailConfigured: Boolean(process.env.ESCROW_DEFAULT_SELLER_EMAIL),
    feePayer: process.env.ESCROW_FEE_PAYER
  });
}
