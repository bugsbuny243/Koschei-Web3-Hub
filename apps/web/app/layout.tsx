import type { Metadata } from "next";
import "./globals.css";
import { SiteChrome } from "./site-chrome";

export const metadata: Metadata = {
  title: "TradePi Globall Machinery",
  description:
    "Quote-based B2B machinery supply and RFQ workflow for agricultural processing equipment.",
};

export default function RootLayout({ children }: { children: React.ReactNode }) {
  return (
    <html lang="tr">
      <body><SiteChrome>{children}</SiteChrome></body>
    </html>
  );
}
