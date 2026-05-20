import "./globals.css";
import type { Metadata } from "next";
import Link from "next/link";
import { Providers } from "@/components/providers";

export const metadata: Metadata = {
  title: "TradePi Globall Machinery",
  description: "B2B dropshipping and RFQ platform for grain processing and seed cleaning machinery."
};

const nav: Array<[string, string]> = [
  ["Home", "/"],
  ["Products", "/products"],
  ["Request Quote", "/request-quote"],
  ["About Supplier", "/about-supplier"],
  ["Contact", "/contact"]
];

export default function RootLayout({ children }: { children: React.ReactNode }) {
  return (
    <html lang="en">
      <body className="bg-slate-50 text-slate-900">
        <Providers>
          <header className="border-b bg-white">
            <div className="mx-auto flex max-w-6xl flex-wrap items-center justify-between gap-3 px-4 py-4 sm:px-6">
              <Link href="/" className="text-lg font-bold">TradePi Globall Machinery</Link>
              <nav className="flex flex-wrap gap-3 text-sm">
                {nav.map(([label, href]) => <Link key={href} href={href} className="rounded px-2 py-1 hover:bg-slate-100">{label}</Link>)}
              </nav>
            </div>
          </header>
          <div className="min-h-screen">{children}</div>
        </Providers>
      </body>
    </html>
  );
}
