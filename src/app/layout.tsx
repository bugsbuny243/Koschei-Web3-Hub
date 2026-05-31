import type { Metadata } from "next"; import "./globals.css";
const description="AI-powered Web3 operating layer for builders, metadata, game assets, risk transparency and ChainOps.";
export const metadata: Metadata={title:"Koschei Web3 Hub",description,openGraph:{title:"Koschei Web3 Hub",description,type:"website"}};
export default function RootLayout({children}:Readonly<{children:React.ReactNode}>){return <html lang="en"><body>{children}</body></html>}
