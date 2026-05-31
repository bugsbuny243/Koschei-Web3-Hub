import type { Metadata } from "next";
import "./globals.css";

export const metadata: Metadata = {
  title: "TeklifPilot | İngilizce İhracat Teklifleri",
  description: "WhatsApp'tan gelen ürün taleplerini 5 dakikada profesyonel İngilizce ihracat teklifine çevirin.",
};

export default function RootLayout({ children }: Readonly<{ children: React.ReactNode }>) {
  return (
    <html lang="tr">
      <body>{children}</body>
    </html>
  );
}
