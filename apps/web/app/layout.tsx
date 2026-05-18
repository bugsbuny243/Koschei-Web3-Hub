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
        <Providers>{children}</Providers>
      </body>
    </html>
  );
}
