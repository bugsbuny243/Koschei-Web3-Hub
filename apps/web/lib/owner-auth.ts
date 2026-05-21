import "server-only";

import { cookies } from "next/headers";

export const OWNER_SESSION_COOKIE = "tradepi_owner_session";

export function isValidAdminPassword(password: string | null | undefined) {
  return !!process.env.ADMIN_PASSWORD && password === process.env.ADMIN_PASSWORD;
}

export async function isOwnerAuthenticated(fallbackPassword?: string | null) {
  const cookieStore = await cookies();
  const session = cookieStore.get(OWNER_SESSION_COOKIE)?.value;
  if (session === "valid") return true;
  return isValidAdminPassword(fallbackPassword ?? null);
}
