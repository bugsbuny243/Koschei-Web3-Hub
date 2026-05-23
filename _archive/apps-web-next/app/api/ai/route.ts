import { NextRequest } from "next/server";
import { ensureUsersTable, getUserFromAuthHeader } from "@/lib/auth";

const COST_BY_TYPE: Record<string, number> = {
  code: 6,
  chat: 3,
  image: 10,
  video: 20,
  audio: 8,
};

function detectType(input: string) {
  const lower = input.toLowerCase();
  if (/(write code|typescript|python|function|bug|refactor|next\.js|sql|kod|kodla)/i.test(lower)) return "code";
  if (/(image|draw|logo|illustration|fotoğraf|resim|görsel)/i.test(lower)) return "image";
  if (/(video|cinematic|movie|clip|reel|kısa video)/i.test(lower)) return "video";
  if (/(voice|speech|audio|transcribe|ses|konuşma)/i.test(lower)) return "audio";
  return "chat";
}

function modelForType(type: string) {
  switch (type) {
    case "code":
      return process.env.TOGETHER_MODEL;
    case "image":
      return process.env.TOGETHER_MODEL_IMAGE;
    case "video":
      return process.env.TOGETHER_MODEL_VIDEO;
    case "audio":
      return process.env.TOGETHER_MODEL_TTS;
    default:
      return process.env.TOGETHER_MODEL_COMPLEX;
  }
}

export async function POST(req: NextRequest) {
  try {
    const authUser = await getUserFromAuthHeader(req.headers.get("authorization"));
    if (!authUser) return new Response(JSON.stringify({ error: "Unauthorized" }), { status: 401 });

    const { prompt, type } = await req.json();
    if (!prompt?.trim()) return new Response(JSON.stringify({ error: "Prompt required" }), { status: 400 });

    const requestType = type || detectType(prompt);
    const model = modelForType(requestType);
    if (!process.env.TOGETHER_API_KEY || !model) {
      return new Response(JSON.stringify({ error: "Together model configuration missing" }), { status: 500 });
    }

    const db = await ensureUsersTable();
    const cost = COST_BY_TYPE[requestType] ?? 4;
    const debitResult = await db.query(
      `UPDATE users SET credits = credits - $1 WHERE id = $2 AND credits >= $1 RETURNING credits`,
      [cost, authUser.sub],
    );
    if (!debitResult.rowCount) return new Response(JSON.stringify({ error: "Insufficient credits" }), { status: 402 });

    const togetherRes = await fetch("https://api.together.xyz/v1/chat/completions", {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        Authorization: `Bearer ${process.env.TOGETHER_API_KEY}`,
      },
      body: JSON.stringify({
        model,
        messages: [{ role: "user", content: prompt }],
        stream: true,
      }),
    });

    if (!togetherRes.ok || !togetherRes.body) {
      await db.query(`UPDATE users SET credits = credits + $1 WHERE id = $2`, [cost, authUser.sub]);
      const details = await togetherRes.text();
      console.error("Together API error:", JSON.stringify({
        status: togetherRes.status,
        model: model,
        hasApiKey: !!process.env.TOGETHER_API_KEY,
        error: details,
      }));
      return new Response(JSON.stringify({ error: "Together API failed", details }), { status: 502 });
    }

    const remainingCredits = debitResult.rows[0].credits;
    const stream = new ReadableStream({
      start(controller) {
        controller.enqueue(new TextEncoder().encode(JSON.stringify({ type: requestType, credits: remainingCredits }) + "\n"));
        const reader = togetherRes.body!.getReader();
        const pump = (): Promise<void> =>
          reader.read().then(({ done, value }) => {
            if (done) {
              controller.close();
              return;
            }
            controller.enqueue(value);
            return pump();
          });
        void pump().catch((error) => controller.error(error));
      },
    });

    return new Response(stream, {
      headers: {
        "Content-Type": "text/event-stream",
        "Cache-Control": "no-cache",
        Connection: "keep-alive",
      },
    });
  } catch (error) {
    console.error("ai route error", error);
    return new Response(JSON.stringify({ error: "AI request failed" }), { status: 500 });
  }
}


export async function GET() {
  return Response.json({
    hasApiKey: !!process.env.TOGETHER_API_KEY,
    model: process.env.TOGETHER_MODEL,
    modelComplex: process.env.TOGETHER_MODEL_COMPLEX,
  });
}
