import "server-only";
import { togetherChatJson } from "@/lib/ai/together-client";

type SourceInput = { title: string; url: string; snippet: string; platform: string };

export type SupplierAiAnalysis = {
  company_name: string | null;
  possible_company_name?: string | null;
  platform: string;
  source_url: string;
  country: string;
  city: string;
  likely_manufacturer: boolean;
  likely_trader: boolean;
  verified_claim_found: boolean;
  product_fit: "high" | "medium" | "low" | "unknown";
  manufacturer_score: number;
  risk_score: number;
  contact_possible: boolean;
  contact_method: "platform" | "email" | "website" | "whatsapp" | "unknown";
  risk_notes: string;
  recommended_action: string;
  confidence: "low" | "medium" | "high";
};

export async function analyzeSupplierLead(source: SourceInput): Promise<SupplierAiAnalysis> {
  return togetherChatJson<SupplierAiAnalysis>([
    { role: "system", content: "You are a cautious B2B supplier risk analyst. Use only provided source data. Return valid JSON only." },
    {
      role: "user",
      content: `Analyze this supplier lead from public search result only:\n${JSON.stringify(source)}\nRules: Never invent manufacturer status, verification, contact details, prices, or certifications. If evidence is weak, confidence must be low and risk_notes must explain why. If company name is unclear, keep company_name as null and use possible_company_name for weak guess.`,
    },
  ]);
}

export function buildOutreachMessage(input: { productCategory?: string | null; platform?: string | null; companyName?: string | null; sourceUrl?: string | null; }) {
  return {
    subject: `Turkey B2B ${input.productCategory ?? "Machinery"} Cooperation Inquiry`,
    body: `Hello ${input.companyName ?? "Supplier Team"},\n\nMy name is Onur Sel from TradePi Globall Machinery.\n\nWe are building a Turkey-focused B2B sourcing workflow for machinery categories and are currently evaluating ${input.productCategory ?? "industrial machinery"} suppliers discovered on ${input.platform ?? "your platform"}.\n\nReference source: ${input.sourceUrl ?? "N/A"}\nTarget market: Turkey\n\nCould you please confirm the points below:\n\n1. Are you a direct manufacturer?\n2. Can you support Turkish B2B buyers?\n3. Can you provide DDP door-to-door quotation to Turkey?\n4. Can you provide clean product photos/videos?\n5. Can you provide production time, warranty, spare parts, and export documents?\n6. Can you work with escrow-secured buyer workflow?\n\nIf aligned, we would like to continue with a professional and transparent onboarding process.\n\nBest regards,\nOnur Sel\nTradePi Globall Machinery`,
  };
}
