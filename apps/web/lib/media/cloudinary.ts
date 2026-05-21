import "server-only";
import crypto from "node:crypto";

type CloudinaryUploadResult = {
  public_id: string;
  secure_url: string;
  original_filename?: string;
};

function getEnv() {
  const cloudName = process.env.CLOUDINARY_CLOUD_NAME;
  const apiKey = process.env.CLOUDINARY_API_KEY;
  const apiSecret = process.env.CLOUDINARY_API_SECRET;
  if (!cloudName || !apiKey || !apiSecret) throw new Error("Missing Cloudinary env variables");
  return { cloudName, apiKey, apiSecret };
}

function signParams(params: Record<string, string>, apiSecret: string) {
  const toSign = Object.keys(params).sort().map((k) => `${k}=${params[k]}`).join("&");
  return crypto.createHash("sha1").update(`${toSign}${apiSecret}`).digest("hex");
}

export async function uploadProductImage(file: File, productSlug: string): Promise<CloudinaryUploadResult> {
  const { cloudName, apiKey, apiSecret } = getEnv();
  const timestamp = Math.floor(Date.now() / 1000).toString();
  const folder = `tradepi/machinery/products/${productSlug}`;
  const signature = signParams({ folder, timestamp }, apiSecret);

  const form = new FormData();
  form.set("file", new Blob([Buffer.from(await file.arrayBuffer())]), file.name);
  form.set("folder", folder);
  form.set("api_key", apiKey);
  form.set("timestamp", timestamp);
  form.set("signature", signature);

  const res = await fetch(`https://api.cloudinary.com/v1_1/${cloudName}/image/upload`, { method: "POST", body: form });
  const body = await res.json();
  if (!res.ok) throw new Error(body?.error?.message ?? "Cloudinary upload failed");
  return body as CloudinaryUploadResult;
}

export async function deleteProductImage(publicId: string): Promise<void> {
  const { cloudName, apiKey, apiSecret } = getEnv();
  const timestamp = Math.floor(Date.now() / 1000).toString();
  const signature = signParams({ public_id: publicId, timestamp }, apiSecret);

  const form = new URLSearchParams();
  form.set("public_id", publicId);
  form.set("api_key", apiKey);
  form.set("timestamp", timestamp);
  form.set("signature", signature);

  const res = await fetch(`https://api.cloudinary.com/v1_1/${cloudName}/image/destroy`, {
    method: "POST",
    headers: { "Content-Type": "application/x-www-form-urlencoded" },
    body: form.toString(),
  });
  if (!res.ok) {
    const body = await res.text();
    throw new Error(`Cloudinary delete failed: ${body}`);
  }
}
