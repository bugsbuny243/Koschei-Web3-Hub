import type { Metadata } from "next";
import "./globals.css";

export const metadata: Metadata = {
  title: "Koschei Web3 Hub",
  description: "AI-powered Web3 operating layer for builders, metadata, game assets, risk transparency and ChainOps.",
};

export default function RootLayout({ children }: Readonly<{ children: React.ReactNode }>) {
  return <html lang="en"><body>{children}</body></html>;
}
