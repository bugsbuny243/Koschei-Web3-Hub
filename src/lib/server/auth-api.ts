import "server-only";

function neonBase() {
  const val = (process.env.EXPO_PUBLIC_NEON_AUTH_URL || process.env.NEON_AUTH_BASE_URL || "").trim().replace(/\/+$/, "");
  if (!val) throw new Error("Neon Auth URL not configured.");
  return val;
}

function mapPath(path: string): string {
  if (path === "/auth/login") return "/sign-in/email";
  if (path === "/auth/signup") return "/sign-up/email";
  if (path === "/auth/me") return "/get-session";
  return path;
}

export async function proxyMemberAuth(request: Request, path: string) {
  let base: string;
  try { base = neonBase(); } catch {
    return new Response(JSON.stringify({ ok: false, message: "Auth is not configured." }), {
      status: 503, headers: { "Content-Type": "application/json" },
    });
  }

  let body: Record<string, unknown> = {};
  try { body = await request.json() as Record<string, unknown>; } catch {}

  const neonPath = mapPath(path);

  if (neonPath === "/sign-up/email" && typeof body.email === "string") {
    body.name = body.name ?? (body.email.split("@")[0] || "User");
  }

  const cookieHeader = request.headers.get("cookie") || "";

  let response: Response;
  try {
    response = await fetch(`${base}${neonPath}`, {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        ...(cookieHeader ? { "Cookie": cookieHeader } : {}),
      },
      body: JSON.stringify(body),
      cache: "no-store",
    });
  } catch {
    return new Response(JSON.stringify({ ok: false, message: "Auth service unavailable." }), {
      status: 502, headers: { "Content-Type": "application/json" },
    });
  }

  const responseBody = await response.text();
  const headers = new Headers({ "Content-Type": "application/json" });
  const setCookies = (response.headers as Headers & { getSetCookie?: () => string[] }).getSetCookie?.() ?? [];
  for (const c of setCookies) {
    headers.append("Set-Cookie", c.replace(/;\s*domain=[^;]+/gi, "").replace(/;\s*path=[^;]+/gi, "") + "; Path=/; SameSite=Lax");
  }
  if (!setCookies.length) {
    const sc = response.headers.get("set-cookie");
    if (sc) headers.append("Set-Cookie", sc.replace(/;\s*domain=[^;]+/gi, "").replace(/;\s*path=[^;]+/gi, "") + "; Path=/; SameSite=Lax");
  }

  return new Response(responseBody || '{"ok":true}', { status: response.status, headers });
}

export type MemberSession = { loggedIn: true; sub: string; email: string };

export async function getMemberSession(cookieHeader: string): Promise<MemberSession | null> {
  let base: string;
  try { base = neonBase(); } catch { throw new Error("Auth not configured."); }

  let response: Response;
  try {
    response = await fetch(`${base}/get-session`, {
      headers: cookieHeader ? { "Cookie": cookieHeader } : {},
      cache: "no-store",
    });
  } catch { throw new Error("Auth request failed."); }

  if (!response.ok) return null;

  const body = await response.json() as Record<string, unknown>;
  const user = (body.user ?? body.data ?? body) as Record<string, unknown> | null;
  if (!user || typeof user !== "object") return null;
  const email = typeof user.email === "string" ? user.email.trim().toLowerCase() : null;
  const sub = typeof user.id === "string" ? user.id.trim() : (typeof user.sub === "string" ? user.sub.trim() : email);
  if (!email || !sub) return null;
  return { loggedIn: true, sub, email };
}
