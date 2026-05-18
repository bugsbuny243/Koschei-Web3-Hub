export type TokenSymbol = "USDC" | "ETH" | "WBTC";

export interface YieldSuggestion {
  protocol: "Aave" | "Compound";
  apy: number;
  token: TokenSymbol;
  reasoning: string;
}

export interface AgentState {
  prompt: string;
  walletAddress: `0x${string}`;
  walletBalance?: string;
  tokenPrice?: string;
  bestYield?: YieldSuggestion;
  swapResult?: string;
  summary?: string;
}
