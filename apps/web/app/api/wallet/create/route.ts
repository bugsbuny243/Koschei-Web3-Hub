import { NextResponse } from "next/server";
import { auth } from "@/lib/auth";
import { prisma } from "@/lib/prisma";
import { encryptPrivateKey } from "@koschei/shared";
import { Wallet } from "ethers";

export async function POST() {
  const session = await auth();
  if (!session?.user?.email) return NextResponse.json({ error: "Unauthorized" }, { status: 401 });

  const user = await prisma.user.findUnique({ where: { email: session.user.email } });
  if (!user) return NextResponse.json({ error: "Unauthorized" }, { status: 401 });

  const existing = await prisma.wallet.findUnique({ where: { userId: user.id } });
  if (existing) {
    return NextResponse.json({ address: existing.address, chain: existing.chain });
  }

  const created = Wallet.createRandom();
  const encryptionKey = process.env.KOSCHEI_WALLET_ENCRYPTION_KEY;
  if (!encryptionKey) return NextResponse.json({ error: "Missing encryption key" }, { status: 500 });

  const encryptedPrivateKey = encryptPrivateKey(created.privateKey, encryptionKey);

  const wallet = await prisma.wallet.create({
    data: {
      userId: user.id,
      address: created.address,
      chain: "base-sepolia",
      walletType: "custodial-invisible",
      encryptedPrivateKey
    }
  });

  await prisma.walletEvent.create({
    data: { walletId: wallet.id, eventType: "wallet_created", metadata: { chain: wallet.chain } }
  });

  return NextResponse.json({ address: wallet.address, chain: wallet.chain });
}
