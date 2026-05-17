import crypto from "crypto";
import dotenv from "dotenv";

dotenv.config();

type EncryptedPayload = {
  iv: string;
  authTag: string;
  cipherText: string;
};

const KEY_BYTES = 32;

function getEncryptionKey(): Buffer {
  const key = process.env.KOSCHEI_WALLET_ENCRYPTION_KEY;
  if (!key) throw new Error("KOSCHEI_WALLET_ENCRYPTION_KEY is missing");

  const keyBuffer = /^[0-9a-fA-F]{64}$/.test(key)
    ? Buffer.from(key, "hex")
    : Buffer.from(key, "utf8");

  if (keyBuffer.length !== KEY_BYTES) {
    throw new Error("KOSCHEI_WALLET_ENCRYPTION_KEY must resolve to exactly 32 bytes");
  }

  return keyBuffer;
}

export function encryptSecret(plainText: string): string {
  if (!plainText) throw new Error("plainText cannot be empty");

  const key = getEncryptionKey();
  const iv = crypto.randomBytes(12);
  const cipher = crypto.createCipheriv("aes-256-gcm", key, iv);

  const encrypted = Buffer.concat([cipher.update(plainText, "utf8"), cipher.final()]);
  const authTag = cipher.getAuthTag();

  const payload: EncryptedPayload = {
    iv: iv.toString("base64"),
    authTag: authTag.toString("base64"),
    cipherText: encrypted.toString("base64")
  };

  return Buffer.from(JSON.stringify(payload), "utf8").toString("base64");
}

export function decryptSecret(encryptedPayload: string): string {
  try {
    const key = getEncryptionKey();
    const decoded = Buffer.from(encryptedPayload, "base64").toString("utf8");
    const payload = JSON.parse(decoded) as EncryptedPayload;

    if (!payload.iv || !payload.authTag || !payload.cipherText) {
      throw new Error("Malformed encrypted payload");
    }

    const decipher = crypto.createDecipheriv("aes-256-gcm", key, Buffer.from(payload.iv, "base64"));
    decipher.setAuthTag(Buffer.from(payload.authTag, "base64"));

    const plain = Buffer.concat([
      decipher.update(Buffer.from(payload.cipherText, "base64")),
      decipher.final()
    ]);

    return plain.toString("utf8");
  } catch (error) {
    const msg = error instanceof Error ? error.message : "unknown decryption error";
    throw new Error(`Unable to decrypt secret: ${msg}`);
  }
}
