import { NextResponse } from "next/server";
import { generateEnglishOfferText, generateFollowUpMessage } from "@/lib/quote";
import type { BuyerInfo, CompanyInfo, ProductInfo } from "@/lib/types";

interface GenerateQuoteRequest {
  company: CompanyInfo;
  buyer: BuyerInfo;
  product: ProductInfo;
}

interface GeneratedQuoteText {
  englishOfferText: string;
  followUpMessage: string;
  source: "together" | "template";
}

interface TogetherChatResponse {
  choices?: Array<{ message?: { content?: string | null } }>;
}

function createTemplateResponse({ company, buyer, product }: GenerateQuoteRequest): GeneratedQuoteText {
  return {
    englishOfferText: generateEnglishOfferText(company, buyer, product),
    followUpMessage: generateFollowUpMessage(buyer, product),
    source: "template",
  };
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null;
}

function hasStrings(value: Record<string, unknown>, keys: string[]) {
  return keys.every((key) => typeof value[key] === "string");
}

function isGenerateQuoteRequest(value: unknown): value is GenerateQuoteRequest {
  if (!isRecord(value) || !isRecord(value.company) || !isRecord(value.buyer) || !isRecord(value.product)) return false;

  return hasStrings(value.company, ["name", "contactPerson", "email", "phone", "address", "website"])
    && hasStrings(value.buyer, ["company", "contactName", "country", "email"])
    && hasStrings(value.product, ["name", "category", "descriptionTr", "unit", "currency", "incoterm", "deliveryTime", "paymentTerms", "packagingDetails", "notes"])
    && typeof value.product.quantity === "number"
    && typeof value.product.unitPrice === "number"
    && typeof value.product.validityDays === "number";
}

function isTogetherEnabled() {
  const provider = process.env.AI_PROVIDER?.trim().toLowerCase();
  return process.env.AI_ENABLED?.trim().toLowerCase() === "true"
    && Boolean(process.env.TOGETHER_API_KEY)
    && (!provider || provider === "together");
}

function parseTogetherContent(content: string): Omit<GeneratedQuoteText, "source"> | null {
  try {
    const parsed: unknown = JSON.parse(content);
    if (!isRecord(parsed) || typeof parsed.englishOfferText !== "string" || typeof parsed.followUpMessage !== "string") return null;

    const englishOfferText = parsed.englishOfferText.trim();
    const followUpMessage = parsed.followUpMessage.trim();
    if (!englishOfferText || !followUpMessage) return null;

    return { englishOfferText, followUpMessage };
  } catch {
    return null;
  }
}

async function generateWithTogether(request: GenerateQuoteRequest): Promise<GeneratedQuoteText | null> {
  if (!isTogetherEnabled()) return null;

  const model = process.env.TOGETHER_MODEL?.trim();
  if (!model) return null;

  try {
    const response = await fetch("https://api.together.ai/v1/chat/completions", {
      method: "POST",
      headers: {
        Authorization: `Bearer ${process.env.TOGETHER_API_KEY}`,
        "Content-Type": "application/json",
      },
      body: JSON.stringify({
        model,
        messages: [
          {
            role: "system",
            content: "You write concise, professional English export quotations for Turkish SMEs. Return only a JSON object with exactly two string fields: englishOfferText and followUpMessage. Do not invent product facts, certifications, prices, delivery terms, or contact details. The offer text must be suitable for a formal quotation. The follow-up must be suitable for WhatsApp or email.",
          },
          {
            role: "user",
            content: `Create quotation text from this JSON input:\n${JSON.stringify(request)}`,
          },
        ],
        response_format: { type: "json_object" },
        temperature: 0.2,
        max_tokens: 700,
      }),
      signal: AbortSignal.timeout(12_000),
    });

    if (!response.ok) return null;

    const data = await response.json() as TogetherChatResponse;
    const content = data.choices?.[0]?.message?.content;
    if (!content) return null;

    const generatedText = parseTogetherContent(content);
    return generatedText ? { ...generatedText, source: "together" } : null;
  } catch {
    return null;
  }
}

export async function POST(request: Request) {
  let payload: unknown;
  try {
    payload = await request.json();
  } catch {
    return NextResponse.json({ error: "Invalid JSON request body." }, { status: 400 });
  }

  if (!isGenerateQuoteRequest(payload)) {
    return NextResponse.json({ error: "Invalid quote details." }, { status: 400 });
  }

  const generatedQuote = await generateWithTogether(payload);
  return NextResponse.json(generatedQuote ?? createTemplateResponse(payload));
}
