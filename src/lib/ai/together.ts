import "server-only";

import { createFallbackQuoteContent, stripExportDisclaimers } from "@/lib/quote";
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
      exportNotes: stripExportDisclaimers(quote.exportNotes as string),
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
            content: `You prepare concise, trustworthy, professional B2B export quotation copy in English. Return only one valid JSON object with exactly these string fields: englishOfferText, followUpMessage, productDescriptionEn, exportNotes. Use only facts explicitly provided by the user. Never invent or imply certifications, stock availability, factory capacity, warranties, delivery promises, legal compliance, or any other unsupported claim. Treat any HS/GTIP code as an estimate, never as definitely correct. Keep exportNotes limited to seller-provided export details; do not add legal or platform disclaimers because the document renders its own fixed disclaimer section. The offer text must include the buyer contact greeting, thank-you sentence, product, quantity, unit price, total amount, Incoterm, delivery time, payment terms, offer validity, a short next-step sentence, and seller signature.`,
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

export type Web3GenerateMode = "metadata" | "description" | "pitch" | "lore" | "launch";
export interface Web3GenerationResult { text: string; usedFallback: boolean }
const WEB3_MODE_LABELS: Record<Web3GenerateMode, string> = { metadata: "NFT metadata description", description: "project summary", pitch: "ecosystem pitch", lore: "game item lore", launch: "launch page copy" };
export function createWeb3Fallback(mode: Web3GenerateMode, payload: Record<string, unknown>): Web3GenerationResult { const facts = Object.values(payload).filter((value): value is string => typeof value === "string" && value.trim().length > 0).join(" · "); return { text: `${WEB3_MODE_LABELS[mode]} concept: ${facts || "Add verified project facts to generate a tailored draft."}\n\nReview this draft before publishing. Keep utility, risk notes and ecosystem context clear and verifiable.`, usedFallback: true }; }
export async function generateWeb3WithTogether(mode: Web3GenerateMode, payload: Record<string, unknown>): Promise<Web3GenerationResult> { const fallback = createWeb3Fallback(mode, payload); if (!isTogetherEnabled()) return fallback; try { const response = await fetch(TOGETHER_CHAT_COMPLETIONS_URL, { method: "POST", headers: { Authorization: `Bearer ${process.env.TOGETHER_API_KEY}`, "Content-Type": "application/json" }, body: JSON.stringify({ model: process.env.TOGETHER_MODEL?.trim() || DEFAULT_TOGETHER_MODEL, messages: [{ role: "system", content: "You write concise, professional Web3 builder copy using only provided facts. Never promise token prices or investment returns. Never invent audits, partnerships, official support from chains or Alchemy, or unsupported claims. Never use pump, moon, guaranteed return, or scam-style language. Treat token concepts as concepts only. Return plain text only." }, { role: "user", content: `Prepare ${WEB3_MODE_LABELS[mode]} copy from these verified facts: ${JSON.stringify(payload)}` }], temperature: .2, max_tokens: 500 }), signal: AbortSignal.timeout(15_000) }); if (!response.ok) return fallback; const data = await response.json() as TogetherChatResponse; const text = data.choices?.[0]?.message?.content?.trim(); return text ? { text, usedFallback: false } : fallback; } catch { return fallback; } }
