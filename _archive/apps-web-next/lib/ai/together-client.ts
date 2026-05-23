const TOGETHER_API_URL = "https://api.together.xyz/v1/chat/completions";

export const DEFAULT_TOGETHER_MODEL = "Qwen/Qwen3-Coder-480B-A35B-Instruct-FP8";

export type TogetherMessage = {
  role: "system" | "user" | "assistant";
  content: string;
};

export async function togetherChatJson<T>(messages: TogetherMessage[], options?: { temperature?: number; maxTokens?: number }): Promise<T> {
  const apiKey = process.env.TOGETHER_API_KEY;
  if (!apiKey) {
    throw new Error("TOGETHER_API_KEY is not configured");
  }

  const model = process.env.TOGETHER_MODEL || DEFAULT_TOGETHER_MODEL;
  const res = await fetch(TOGETHER_API_URL, {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
      Authorization: `Bearer ${apiKey}`,
    },
    body: JSON.stringify({
      model,
      messages,
      temperature: options?.temperature ?? 0.1,
      max_tokens: options?.maxTokens ?? 1600,
      response_format: { type: "json_object" },
    }),
    cache: "no-store",
  });

  if (!res.ok) {
    const text = await res.text();
    throw new Error(`Together API request failed (${res.status}): ${text}`);
  }

  const data = (await res.json()) as {
    choices?: Array<{ message?: { content?: string } }>;
  };

  const content = data.choices?.[0]?.message?.content;
  if (!content) {
    throw new Error("Together API returned empty content");
  }

  return JSON.parse(content) as T;
}
