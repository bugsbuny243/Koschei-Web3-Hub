import { NextResponse } from "next/server";

export async function POST() {
  return NextResponse.json({ ok: false, error: "deprecated_route", detail: "This legacy agent route is disabled in production MVP." }, { status: 410 });
}
