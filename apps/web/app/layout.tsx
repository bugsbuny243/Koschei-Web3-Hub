import "./globals.css";
import type { Metadata } from "next";
import { Providers } from "@/components/providers";

export const metadata: Metadata = {
  title: "Koscei Bridge",
  description: "Create your AI Agent"
};

export default function RootLayout({ children }: { children: React.ReactNode }) {
  return (
    <html lang="en">
      <body>
        <Providers>
          <div className="min-h-screen">
            {children}
            <footer className="mt-12 border-t bg-gray-50">
              <div className="mx-auto flex max-w-6xl flex-col gap-3 p-6 text-sm text-gray-700 md:flex-row md:items-center md:justify-between">
                <p>Voluntary support helps sustain ongoing Koschei Web3 Game Bridge development.</p>
                <a
                  href="https://www.shopier.com/TradeVisual/47208457"
                  target="_blank"
                  rel="noopener noreferrer"
                  className="inline-flex w-fit rounded border border-emerald-700 px-3 py-2 font-semibold text-emerald-700 hover:bg-emerald-100"
                >
                  Support with 10 TL
                </a>
              </div>
            </footer>
          </div>
        </Providers>
      </body>
    </html>
  );
}
