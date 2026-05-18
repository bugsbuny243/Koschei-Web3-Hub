import { NextResponse } from "next/server";

function notImplemented() {
  return NextResponse.json(
    { error: "Auth is not enabled in this MVP yet." },
    { status: 501 }
  );
}

export async function GET() {
  return notImplemented();
}

export async function POST() {
  return notImplemented();
}
