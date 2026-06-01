import { proxyAuthRequest } from "@/lib/server/auth-api/proxy";

// Member credentials are handled only by the Go auth-api service.
export async function POST(request: Request) {
  return proxyAuthRequest(request, "/auth/signup");
}
