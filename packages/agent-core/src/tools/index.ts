import { createPublicClient, http, parseEther, type Address } from "viem";
import { arbitrum } from "viem/chains";
import type { TokenSymbol, YieldSuggestion } from "../types";

const client = createPublicClient({ chain: arbitrum, transport: http(process.env.ARBITRUM_RPC_URL) });

export async function getWalletBalance(walletAddress: Address) {
  const balance = await client.getBalance({ address: walletAddress });
  return `${Number(balance) / 1e18} ETH`;
}

export async function getTokenPrice(token: TokenSymbol) {
  const mockPrices: Record<TokenSymbol, number> = { USDC: 1, ETH: 3500, WBTC: 62000 };
  return `$${mockPrices[token]} (${token})`;
}

export async function suggestBestYield(token: TokenSymbol): Promise<YieldSuggestion> {
  const options: YieldSuggestion[] = [
    { protocol: "Aave", apy: 7.1, token, reasoning: "Stable liquidity and competitive APY" },
    { protocol: "Compound", apy: 6.4, token, reasoning: "Lower volatility and safer utilization" }
  ];
  return options.sort((a, b) => b.apy - a.apy)[0];
}

export async function executeSwap(from: TokenSymbol, to: TokenSymbol, amountEth: string) {
  const wei = parseEther(amountEth);
  return `Swap simulated on Arbitrum: ${from} -> ${to}, amount=${wei.toString()} wei`;
}
