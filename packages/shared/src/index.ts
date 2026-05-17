import crypto from "crypto";

export type WalletChain = "base-sepolia";
export type WalletType = "custodial-invisible";

export interface WalletPublicView {
  address: string;
  chain: WalletChain;
  walletType: WalletType;
  createdAt: Date;
}

export function encryptPrivateKey(privateKey: string, secret: string): string {
  const iv = crypto.randomBytes(12);
  const key = crypto.createHash("sha256").update(secret).digest();
  const cipher = crypto.createCipheriv("aes-256-gcm", key, iv);
  const encrypted = Buffer.concat([cipher.update(privateKey, "utf8"), cipher.final()]);
  const tag = cipher.getAuthTag();
  return Buffer.concat([iv, tag, encrypted]).toString("base64");
}
