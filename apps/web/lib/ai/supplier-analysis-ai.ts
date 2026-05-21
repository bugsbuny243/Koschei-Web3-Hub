import "server-only";
import { togetherChatJson } from "@/lib/ai/together-client";

type SourceInput = { title: string; url: string; snippet: string; platform: string };

export type SupplierAiAnalysis = {
  company_name: string;
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
      content: `Analyze this supplier lead from public search result only:\n${JSON.stringify(source)}\nRules: Do not invent company names, verification, prices, or contact data. If weak evidence, set confidence low/medium and explain in risk_notes.`,
    },
  ]);
}

export function buildOutreachMessage() {
  return {
    subject: "Turkey B2B Machinery RFQ Cooperation Inquiry - TradePi Globall",
    body: `Hello,\n\nMy name is Onur Sel from TradePi Globall Machinery.\n\nWe are building a Turkey-focused B2B RFQ platform for agricultural machinery and processing equipment. We connect Turkish buyers with reliable manufacturers and coordinate quote-based sourcing, supplier confirmation, and escrow-secured payment workflow.\n\nWe are currently looking for manufacturer partners for products such as seed cleaning machines, grain cleaners, gravity separators, color sorters, bucket elevators, packing scales, and complete seed processing lines.\n\nCould you please confirm:\n\n1. Are you the direct manufacturer?\n2. Can you support Turkey market buyers?\n3. Can you provide DDP door-to-door quotations to Turkey?\n4. Can you provide production time, shipping time, warranty, spare parts and technical documents?\n5. Can you provide clean product photos/videos for Turkish RFQ listings?\n6. Can you work with an escrow-secured international buyer payment process?\n7. Can you provide catalog, quotation terms, and export documents?\n\nWe are interested in long-term cooperation and can send qualified buyer RFQs after supplier verification.\n\nBest regards,\nOnur Sel\nTradePi Globall Machinery`,
  };
}
