import type { Metadata } from "next";
import "./globals.css";
import { SiteChrome } from "./site-chrome";

export const metadata: Metadata = {
  title: "Koschei | The Immortal AI Platform",
  description:
    "Koschei public SaaS platform and private owner command center with one shared AI production engine.",
};

export default function RootLayout({ children }: { children: React.ReactNode }) {
  return (
    <html lang="en">
      <body><SiteChrome>{children}</SiteChrome></body>
    </html>
  );
}
