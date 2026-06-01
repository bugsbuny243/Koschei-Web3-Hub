import { proxyAuthRequest } from "@/lib/server/auth-api/proxy";

export async function POST(request: Request) {
  return proxyAuthRequest(request, "/auth/login");
}
