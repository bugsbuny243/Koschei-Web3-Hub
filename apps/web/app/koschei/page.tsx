"use client";

import { FormEvent, useEffect, useMemo, useRef, useState } from "react";

type MessageType = "chat" | "code" | "image" | "video";

type ChatMessage = {
  id: string;
  role: "user" | "assistant";
  content: string;
  type: MessageType;
};

type ApiChunk = {
  type?: MessageType;
  content?: string;
  creditsRemaining?: number;
};

const codeKeywords = new Set([
  "const",
  "let",
  "var",
  "function",
  "return",
  "if",
  "else",
  "for",
  "while",
  "import",
  "from",
  "export",
  "class",
  "async",
  "await",
  "new",
  "try",
  "catch",
  "finally",
  "switch",
  "case",
  "break",
  "continue",
  "interface",
  "type",
  "extends",
  "implements",
]);

const detectTypeFromContent = (content: string): MessageType => {
  const value = content.trim();

  if (/^(https?:\/\/\S+\.(png|jpe?g|gif|webp|svg))(\?\S+)?$/i.test(value)) {
    return "image";
  }

  if (/^(https?:\/\/\S+\.(mp4|webm|ogg|mov))(\?\S+)?$/i.test(value)) {
    return "video";
  }

  if (/```[\s\S]*```/.test(value) || /(const|function|class|import|export)\s+/.test(value)) {
    return "code";
  }

  return "chat";
};

const renderMarkdownLike = (text: string): string => {
  return text
    .replace(/&/g, "&amp;")
    .replace(/</g, "&lt;")
    .replace(/>/g, "&gt;")
    .replace(/\*\*(.*?)\*\*/g, "<strong>$1</strong>")
    .replace(/\*(.*?)\*/g, "<em>$1</em>")
    .replace(/`([^`]+)`/g, "<code class='rounded bg-zinc-800 px-1 py-0.5'>$1</code>")
    .replace(/\n/g, "<br />");
};

const highlightCode = (code: string): string => {
  return code
    .replace(/&/g, "&amp;")
    .replace(/</g, "&lt;")
    .replace(/>/g, "&gt;")
    .replace(/("[^"]*"|'[^']*')/g, "<span class='text-emerald-300'>$1</span>")
    .replace(/\b(\d+)\b/g, "<span class='text-fuchsia-300'>$1</span>")
    .replace(/\b([A-Za-z_][A-Za-z0-9_]*)\b/g, (match) =>
      codeKeywords.has(match) ? `<span class='text-sky-300 font-semibold'>${match}</span>` : match,
    );
};

export default function KoscheiPage() {
  const [messages, setMessages] = useState<ChatMessage[]>([]);
  const [prompt, setPrompt] = useState("");
  const [creditsRemaining, setCreditsRemaining] = useState<number | null>(null);
  const [isStreaming, setIsStreaming] = useState(false);
  const [token, setToken] = useState<string | null>(null);
  const bottomRef = useRef<HTMLDivElement | null>(null);

  useEffect(() => {
    const jwt = localStorage.getItem("koschei_token");
    if (!jwt) {
      window.location.href = "/auth";
      return;
    }
    setToken(jwt);
  }, []);

  useEffect(() => {
    bottomRef.current?.scrollIntoView({ behavior: "smooth" });
  }, [messages]);

  const chatGroups = useMemo(() => {
    if (messages.length === 0) return [];
    return [{ id: "current", title: "Current Chat", count: messages.length }];
  }, [messages]);

  const sendMessage = async (event: FormEvent) => {
    event.preventDefault();
    if (!prompt.trim() || !token || isStreaming) return;

    const userMessage: ChatMessage = {
      id: crypto.randomUUID(),
      role: "user",
      content: prompt,
      type: "chat",
    };

    const assistantId = crypto.randomUUID();
    setMessages((prev) => [
      ...prev,
      userMessage,
      { id: assistantId, role: "assistant", content: "", type: "chat" },
    ]);

    const input = prompt;
    setPrompt("");
    setIsStreaming(true);

    try {
      const response = await fetch("/api/ai", {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
          Authorization: `Bearer ${token}`,
        },
        body: JSON.stringify({ prompt: input }),
      });

      if (!response.ok || !response.body) {
        throw new Error("Unable to get AI response");
      }

      const reader = response.body.getReader();
      const decoder = new TextDecoder();
      let buffer = "";
      let assistantContent = "";

      while (true) {
        const { done, value } = await reader.read();
        if (done) break;

        buffer += decoder.decode(value, { stream: true });
        const lines = buffer.split("\n");
        buffer = lines.pop() ?? "";

        for (const line of lines) {
          const trimmedLine = line.trim();
          if (!trimmedLine) continue;

          const payload = trimmedLine.startsWith("data:")
            ? trimmedLine.slice(5).trim()
            : trimmedLine;

          if (!payload || payload === "[DONE]") continue;

          let chunk: ApiChunk | null = null;
          try {
            const parsed = JSON.parse(payload) as ApiChunk & {
              choices?: Array<{ delta?: { content?: string } }>;
            };
            const deltaContent = parsed.choices?.[0]?.delta?.content;

            chunk = {
              ...parsed,
              content: typeof deltaContent === "string" ? deltaContent : parsed.content,
            };
          } catch {
            chunk = { content: payload };
          }

          if (typeof chunk.creditsRemaining === "number") {
            setCreditsRemaining(chunk.creditsRemaining);
          }

          if (typeof chunk.content === "string") {
            assistantContent += chunk.content;
            const nextType = chunk.type ?? detectTypeFromContent(assistantContent);
            setMessages((prev) =>
              prev.map((message) =>
                message.id === assistantId
                  ? {
                      ...message,
                      content: assistantContent,
                      type: nextType,
                    }
                  : message,
              ),
            );
          }
        }
      }

      if (buffer.trim()) {
        const trimmedBuffer = buffer.trim();
        const payload = trimmedBuffer.startsWith("data:")
          ? trimmedBuffer.slice(5).trim()
          : trimmedBuffer;

        if (payload && payload !== "[DONE]") {
          let trailingChunk: ApiChunk | null = null;
          try {
            const parsed = JSON.parse(payload) as ApiChunk & {
              choices?: Array<{ delta?: { content?: string } }>;
            };
            const deltaContent = parsed.choices?.[0]?.delta?.content;
            trailingChunk = {
              ...parsed,
              content: typeof deltaContent === "string" ? deltaContent : parsed.content,
            };
          } catch {
            trailingChunk = { content: payload };
          }

          if (typeof trailingChunk.creditsRemaining === "number") {
            setCreditsRemaining(trailingChunk.creditsRemaining);
          }

          if (typeof trailingChunk.content === "string") {
            assistantContent += trailingChunk.content;
            const nextType = trailingChunk.type ?? detectTypeFromContent(assistantContent);
            setMessages((prev) =>
              prev.map((message) =>
                message.id === assistantId
                  ? {
                      ...message,
                      content: assistantContent,
                      type: nextType,
                    }
                  : message,
              ),
            );
          }
        }
      }
    } catch (error) {
      const message = error instanceof Error ? error.message : "Unknown error";
      setMessages((prev) =>
        prev.map((entry) =>
          entry.id === assistantId
            ? { ...entry, content: `Error: ${message}`, type: "chat" }
            : entry,
        ),
      );
    } finally {
      setIsStreaming(false);
    }
  };

  return (
    <div className="flex h-screen w-screen bg-zinc-950 text-zinc-100">
      <aside className="flex w-72 flex-col border-r border-zinc-800 bg-zinc-900/80 p-4 md:w-80">
        <h1 className="mb-5 text-xl font-semibold">Koschei AI</h1>

        <div className="mb-4 rounded-xl border border-zinc-800 bg-zinc-900 p-3">
          <p className="text-xs uppercase tracking-wide text-zinc-400">Credits remaining</p>
          <p className="text-lg font-medium">{creditsRemaining ?? "—"}</p>
        </div>

        <div className="min-h-0 flex-1 overflow-y-auto">
          <p className="mb-2 text-xs uppercase tracking-wide text-zinc-400">Chat history</p>
          <div className="space-y-2">
            {chatGroups.length === 0 ? (
              <p className="text-sm text-zinc-500">No chats yet.</p>
            ) : (
              chatGroups.map((group) => (
                <button
                  key={group.id}
                  type="button"
                  className="w-full rounded-lg border border-zinc-800 bg-zinc-900 px-3 py-2 text-left text-sm hover:bg-zinc-800"
                >
                  <p>{group.title}</p>
                  <p className="text-xs text-zinc-400">{group.count} messages</p>
                </button>
              ))
            )}
          </div>
        </div>

        <button
          type="button"
          onClick={() => {
            localStorage.removeItem("koschei_token");
            window.location.href = "/auth";
          }}
          className="mt-4 rounded-lg bg-red-600/90 px-4 py-2 text-sm font-medium hover:bg-red-500"
        >
          Logout
        </button>
      </aside>

      <main className="flex min-w-0 flex-1 flex-col">
        <section className="flex-1 overflow-y-auto p-4 md:p-6">
          <div className="mx-auto flex w-full max-w-4xl flex-col gap-4">
            {messages.map((message) => (
              <div
                key={message.id}
                className={`rounded-xl border p-4 max-w-full overflow-x-hidden ${
                  message.role === "user"
                    ? "ml-auto max-w-[90%] border-sky-700/60 bg-sky-900/20"
                    : "mr-auto max-w-[95%] border-zinc-800 bg-zinc-900/80"
                }`}
              >
                {message.type === "code" ? (
                  <pre
                    className="overflow-x-auto rounded-lg bg-zinc-950 p-3 text-sm leading-relaxed"
                    dangerouslySetInnerHTML={{ __html: highlightCode(message.content.replace(/```/g, "")) }}
                  />
                ) : message.type === "image" ? (
                  <img src={message.content} alt="AI generated response" className="h-auto max-w-full rounded-lg" />
                ) : message.type === "video" ? (
                  <video src={message.content} controls className="h-auto max-w-full rounded-lg" />
                ) : (
                  <div
                    className="prose prose-invert max-w-none text-sm whitespace-pre-wrap break-words"
                    style={{ wordWrap: "break-word", overflowWrap: "break-word", maxWidth: "100%", overflowX: "hidden" }}
                    dangerouslySetInnerHTML={{ __html: renderMarkdownLike(message.content) }}
                  />
                )}
              </div>
            ))}
            <div ref={bottomRef} />
          </div>
        </section>

        <form onSubmit={sendMessage} className="border-t border-zinc-800 bg-zinc-900/90 p-3 md:p-4">
          <div className="mx-auto flex w-full max-w-4xl gap-2">
            <input
              value={prompt}
              onChange={(event) => setPrompt(event.target.value)}
              placeholder="Ask Koschei AI anything..."
              className="flex-1 rounded-xl border border-zinc-700 bg-zinc-950 px-4 py-3 text-sm outline-none ring-sky-500 placeholder:text-zinc-500 focus:ring-2"
              disabled={isStreaming}
            />
            <button
              type="submit"
              disabled={isStreaming || !prompt.trim()}
              className="rounded-xl bg-sky-600 px-5 py-3 text-sm font-medium hover:bg-sky-500 disabled:cursor-not-allowed disabled:opacity-50"
            >
              {isStreaming ? "Streaming..." : "Send"}
            </button>
          </div>
        </form>
      </main>
    </div>
  );
}
