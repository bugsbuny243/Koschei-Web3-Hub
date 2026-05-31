import { NextResponse } from "next/server";
import { generateQuoteWithTogether } from "@/lib/ai/together";
import type { QuoteFormData } from "@/lib/types";

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null;
}

function hasStrings(value: Record<string, unknown>, keys: string[]) {
  return keys.every((key) => typeof value[key] === "string");
}

function isQuoteFormData(value: unknown): value is QuoteFormData {
  if (!isRecord(value) || !isRecord(value.company) || !isRecord(value.buyer) || !isRecord(value.product)) return false;

  return hasStrings(value.company, ["name", "contactPerson", "email", "phone", "address", "website"])
    && hasStrings(value.buyer, ["company", "contactName", "country", "email"])
    && hasStrings(value.product, ["name", "category", "descriptionTr", "unit", "currency", "incoterm", "deliveryTime", "paymentTerms", "packagingDetails", "notes"])
    && (value.product.hsCode === undefined || typeof value.product.hsCode === "string")
    && typeof value.product.quantity === "number"
    && typeof value.product.unitPrice === "number"
    && typeof value.product.validityDays === "number";
}

export async function POST(request: Request) {
  let payload: unknown;
  try {
    payload = await request.json();
  } catch {
    return NextResponse.json({ error: "Invalid JSON request body." }, { status: 400 });
  }

  if (!isQuoteFormData(payload)) {
    return NextResponse.json({ error: "Invalid quote details." }, { status: 400 });
  }

  return NextResponse.json(await generateQuoteWithTogether(payload));
}
