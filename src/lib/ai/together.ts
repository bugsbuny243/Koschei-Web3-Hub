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

export type Web3GenerationMode = "metadata" | "description" | "pitch" | "lore";
const WEB3_SAFETY = "Never promise token price, investment returns or guaranteed value. Never invent audits, partnerships or official ecosystem support. Never use pump language. Do not provide legal, financial, investment or security advice. Use only facts supplied by the user. Keep the result professional, concise and infrastructure-focused.";
function fallbackWeb3Text(mode: Web3GenerationMode, payload: Record<string, unknown>) {
  const name=String(payload.assetName || payload.projectName || "This Web3 project"); const description=String(payload.description || "a builder-defined digital asset"); const utility=String(payload.utility || "documented ecosystem utility");
  const templates={metadata:`${name} is ${description}. It is structured for clear metadata discovery and designed around ${utility}. Review all project details and risk notes before publishing.`,description:`${name} is a Web3 asset concept focused on ${utility}. Its metadata is designed to help builders communicate purpose, attributes and risk notes clearly.`,pitch:`${name} is an ecosystem-ready builder concept that uses standardized metadata, transparent utility and reviewable risk notes to support responsible onboarding.`,lore:`Within its digital ecosystem, ${name} represents ${description}. Its role is shaped by ${utility}, with attributes documented for builders and players to review.`}; return templates[mode];
}
export async function generateWeb3WithTogether(mode: Web3GenerationMode,payload:Record<string,unknown>){const fallback={text:fallbackWeb3Text(mode,payload),usedFallback:true};if(!isTogetherEnabled())return fallback;try{const response=await fetch(TOGETHER_CHAT_COMPLETIONS_URL,{method:"POST",headers:{Authorization:`Bearer ${process.env.TOGETHER_API_KEY}`,"Content-Type":"application/json"},body:JSON.stringify({model:process.env.TOGETHER_MODEL?.trim()||DEFAULT_TOGETHER_MODEL,messages:[{role:"system",content:`You write safe Web3 builder infrastructure copy. ${WEB3_SAFETY}`},{role:"user",content:`Generate one ${mode} output as plain text from this builder payload: ${JSON.stringify(payload)}`}],temperature:.2,max_tokens:500}),signal:AbortSignal.timeout(15000)});if(!response.ok)return fallback;const data=await response.json() as TogetherChatResponse;const text=data.choices?.[0]?.message?.content?.trim();return text?{text,usedFallback:false}:fallback}catch{return fallback}}
