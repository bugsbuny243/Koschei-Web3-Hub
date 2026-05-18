"use client";
import { getDefaultConfig } from "@rainbow-me/rainbowkit";
import { arbitrum, baseSepolia } from "wagmi/chains";

export const wagmiConfig: ReturnType<typeof getDefaultConfig> = getDefaultConfig({
  appName: "Koscei Bridge",
  projectId: process.env.NEXT_PUBLIC_WALLETCONNECT_PROJECT_ID || "demo",
  chains: [arbitrum, baseSepolia],
  ssr: true
});
