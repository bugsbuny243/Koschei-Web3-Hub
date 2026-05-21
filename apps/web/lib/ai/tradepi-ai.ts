import { togetherChatJson } from "@/lib/ai/together-client";

type QuoteRequestRow = Record<string, unknown>;

export type AnalyzeQuoteAiResult = {
  language: string;
  normalized_summary_en: string;
  extracted_fields: {
    customer_name: string | null;
    company_name: string | null;
    importer_company_name: string | null;
    importer_tax_id: string | null;
    importer_customs_broker: string | null;
    destination_country: string | null;
    destination_city: string | null;
    requested_product: string | null;
    quantity: string | null;
    requested_trade_term: string | null;
  };
  missing_required_fields: string[];
  risks: string[];
  admin_workflow_text: string;
  internal_risk_notes: string;
};

export type SupplierMessageAiResult = {
  subject: string;
  supplier_message_en: string;
  checklist: string[];
  compliance_notes: string[];
};

const AI_POLICY = `You are TradePi AI Brain for internal admin support.
Hard rules:
- Never invent prices or supplier cost values.
- Never create fake DDP cost.
- Never guarantee delivery timeline (including 75-80 days).
- Never replace Cathy/supplier final decisions.
- If company/importer/customs info is missing, mark as risk.
- Always state Cathy/supplier must confirm DDP, insurance, customs and delivery scope.
- Output valid JSON only.`;

export async function analyzeQuoteRequestWithAi(input: { quoteRequest: QuoteRequestRow }): Promise<AnalyzeQuoteAiResult> {
  return togetherChatJson<AnalyzeQuoteAiResult>([
    { role: "system", content: AI_POLICY },
    {
      role: "user",
      content: `Analyze this Turkish RFQ request and return structured JSON for internal workflow.\nRFQ JSON:\n${JSON.stringify(input.quoteRequest)}`,
    },
    {
      role: "user",
      content:
        "Required JSON keys: language, normalized_summary_en, extracted_fields, missing_required_fields, risks, admin_workflow_text, internal_risk_notes. Include missing customs/importer/company information in missing_required_fields and risks.",
    },
  ]);
}

export async function buildSupplierMessageWithAi(input: { quoteRequest: QuoteRequestRow; analysis?: AnalyzeQuoteAiResult | null }): Promise<SupplierMessageAiResult> {
  return togetherChatJson<SupplierMessageAiResult>([
    { role: "system", content: AI_POLICY },
    {
      role: "user",
      content: `Prepare supplier-ready English request message for Cathy based on RFQ.\nRFQ JSON:\n${JSON.stringify(input.quoteRequest)}\nAnalysis JSON:\n${JSON.stringify(input.analysis || null)}`,
    },
    {
      role: "user",
      content:
        "Required JSON keys: subject, supplier_message_en, checklist, compliance_notes. checklist must include DDP/insurance/customs/delivery-scope confirmation request and missing info questions. No prices.",
    },
  ]);
}
