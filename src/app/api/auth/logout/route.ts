import { proxyMemberAuth } from "@/lib/server/auth-api";

export async function POST(request: Request) {
  return proxyMemberAuth(request, "/auth/logout");
}
