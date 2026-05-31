import type { Metadata } from "next";
import "./globals.css";
export const metadata: Metadata = { title: "Koschei Web3 Hub | AI-powered Web3 operating layer", description: "Builder infrastructure for games, assets, metadata, launch pages and ecosystem growth." };
export default function RootLayout({ children }: Readonly<{ children: React.ReactNode }>) { return <html lang="en"><body>{children}</body></html>; }
