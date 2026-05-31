import { NextResponse } from "next/server";
import { isAdminRequest } from "@/lib/server/admin-auth";
import { listAdminPaymentRequests, PaymentRequestStatus, reviewPaymentRequest } from "@/lib/server/db";

const statuses = new Set<PaymentRequestStatus>(["manual_verified", "rejected"]);

export async function GET() {
  if (!await isAdminRequest()) return NextResponse.json({ error: "Unauthorized." }, { status: 401 });
  try {
    return NextResponse.json({ paymentRequests: await listAdminPaymentRequests() });
  } catch {
    return NextResponse.json({ error: "Admin payment requests are unavailable." }, { status: 503 });
  }
}

export async function PATCH(request: Request) {
  if (!await isAdminRequest()) return NextResponse.json({ error: "Unauthorized." }, { status: 401 });
  let body: Record<string, unknown>;
  try {
    body = await request.json() as Record<string, unknown>;
  } catch {
    return NextResponse.json({ error: "Invalid JSON body." }, { status: 400 });
  }
  if (typeof body.id !== "string" || typeof body.status !== "string" || !statuses.has(body.status as PaymentRequestStatus)) {
    return NextResponse.json({ error: "id and a supported status are required." }, { status: 400 });
  }
  try {
    return NextResponse.json({ paymentRequest: await reviewPaymentRequest(body.id, body.status as PaymentRequestStatus) });
  } catch {
    return NextResponse.json({ error: "Could not review the payment request." }, { status: 400 });
  }
}
