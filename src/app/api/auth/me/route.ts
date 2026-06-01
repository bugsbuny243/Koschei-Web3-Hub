import { proxyAuthRequest } from "@/lib/server/auth-api/proxy";

export async function GET(request: Request) {
  return proxyAuthRequest(request, "/auth/me");
}
