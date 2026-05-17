import { NextResponse } from "next/server";
import { auth } from "@/lib/auth";
import { prisma } from "@/lib/prisma";

export async function GET() {
  const session = await auth();
  if (!session?.user?.email) return NextResponse.json({ error: "Unauthorized" }, { status: 401 });

  const user = await prisma.user.findUnique({ where: { email: session.user.email } });
  if (!user) return NextResponse.json({ error: "Unauthorized" }, { status: 401 });

  const wallet = await prisma.wallet.findUnique({ where: { userId: user.id } });
  if (!wallet) return NextResponse.json({ error: "Wallet not found" }, { status: 404 });

  return NextResponse.json({
    address: wallet.address,
    chain: wallet.chain,
    walletType: wallet.walletType,
    createdAt: wallet.createdAt
  });
}
