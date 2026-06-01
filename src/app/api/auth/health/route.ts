import { proxyMemberAuth } from "@/lib/server/auth-api";

export async function GET(request: Request) {
  return proxyMemberAuth(request, "/health");
}
