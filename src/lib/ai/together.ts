import "server-only";

import { createFallbackQuoteContent, HS_GTIP_DISCLAIMER, PLATFORM_DISCLAIMER } from "@/lib/quote";
import type { GeneratedQuoteContent, QuoteFormData } from "@/lib/types";

const TOGETHER_CHAT_COMPLETIONS_URL = "https://api.together.ai/v1/chat/completions";
const DEFAULT_TOGETHER_MODEL = "Qwen/Qwen3-235B-A22B-Instruct-2507-tput";

interface TogetherChatResponse {
  choices?: Array<{ message?: { content?: string | null } }>;
}

type ModelQuoteContent = Omit<GeneratedQuoteContent, "usedFallback">;

function isTogetherEnabled() {
  return process.env.AI_ENABLED?.trim().toLowerCase() === "true"
    && process.env.AI_PROVIDER?.trim().toLowerCase() === "together"
    && Boolean(process.env.TOGETHER_API_KEY?.trim());
}

function ensureRequiredDisclaimers(exportNotes: string) {
  const notes = exportNotes.trim();
  const required = [HS_GTIP_DISCLAIMER, PLATFORM_DISCLAIMER].filter((disclaimer) => !notes.includes(disclaimer));
  return [notes, ...required].filter(Boolean).join("\n\n");
}

function parseModelContent(content: string): ModelQuoteContent | null {
  try {
    const cleaned = content.trim().replace(/^```(?:json)?\s*/i, "").replace(/\s*```$/, "");
    const parsed: unknown = JSON.parse(cleaned);
    if (typeof parsed !== "object" || parsed === null) return null;

    const quote = parsed as Record<string, unknown>;
    const requiredFields = ["englishOfferText", "followUpMessage", "productDescriptionEn", "exportNotes"];
    if (!requiredFields.every((field) => typeof quote[field] === "string" && quote[field].trim())) return null;

    return {
      englishOfferText: (quote.englishOfferText as string).trim(),
      followUpMessage: (quote.followUpMessage as string).trim(),
      productDescriptionEn: (quote.productDescriptionEn as string).trim(),
      exportNotes: ensureRequiredDisclaimers(quote.exportNotes as string),
    };
  } catch {
    return null;
  }
}

export async function generateQuoteWithTogether(input: QuoteFormData): Promise<GeneratedQuoteContent> {
  const fallback = createFallbackQuoteContent(input.company, input.buyer, input.product);
  if (!isTogetherEnabled()) return fallback;

  try {
    const response = await fetch(TOGETHER_CHAT_COMPLETIONS_URL, {
      method: "POST",
      headers: {
        Authorization: `Bearer ${process.env.TOGETHER_API_KEY}`,
        "Content-Type": "application/json",
      },
      body: JSON.stringify({
        model: process.env.TOGETHER_MODEL?.trim() || DEFAULT_TOGETHER_MODEL,
        messages: [
          {
            role: "system",
            content: `You prepare concise, trustworthy, professional B2B export quotation copy in English. Return only one valid JSON object with exactly these string fields: englishOfferText, followUpMessage, productDescriptionEn, exportNotes. Use only facts explicitly provided by the user. Never invent or imply certifications, stock availability, factory capacity, warranties, delivery promises, legal compliance, or any other unsupported claim. Treat any HS/GTIP code as an estimate, never as definitely correct. Keep these exact sentences in exportNotes:\n${HS_GTIP_DISCLAIMER}\n${PLATFORM_DISCLAIMER}`,
          },
          {
            role: "user",
            content: `Prepare the quotation copy from this seller-provided form data. Keep the tone short, clear, and confidence-building without adding facts:\n${JSON.stringify(input)}`,
          },
        ],
        response_format: { type: "json_object" },
        temperature: 0.2,
        max_tokens: 1200,
      }),
      signal: AbortSignal.timeout(15_000),
    });

    if (!response.ok) return fallback;
    const data = await response.json() as TogetherChatResponse;
    const generated = data.choices?.[0]?.message?.content;
    const parsed = generated ? parseModelContent(generated) : null;
    return parsed ? { ...parsed, usedFallback: false } : fallback;
  } catch {
    return fallback;
  }
}
