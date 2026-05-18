import { StateGraph, START, END } from "@langchain/langgraph";
import { ChatGroq } from "@langchain/groq";
import type { AgentState } from "../types";
import { executeSwap, getTokenPrice, getWalletBalance, suggestBestYield } from "../tools";

const llm = new ChatGroq({ apiKey: process.env.GROQ_API_KEY, model: process.env.GROQ_MODEL || "llama-3.3-70b-versatile" });

export function buildAutoYieldGraph() {
  const graph = new StateGraph<AgentState>({
    channels: {
      prompt: null,
      walletAddress: null,
      walletBalance: null,
      tokenPrice: null,
      bestYield: null,
      swapResult: null,
      summary: null
    }
  });

  graph.addNode("wallet", async (state) => ({ ...state, walletBalance: await getWalletBalance(state.walletAddress) }));
  graph.addNode("price", async (state) => ({ ...state, tokenPrice: await getTokenPrice("ETH") }));
  graph.addNode("yield", async (state) => ({ ...state, bestYield: await suggestBestYield("USDC") }));
  graph.addNode("swap", async (state) => ({ ...state, swapResult: await executeSwap("ETH", "USDC", "0.01") }));
  graph.addNode("summarize", async (state) => {
    const response = await llm.invoke([
      ["system", "You are AutoYieldOptimizerAgent. Return concise Turkish summary."],
      ["human", `Prompt: ${state.prompt}\nBalance: ${state.walletBalance}\nPrice: ${state.tokenPrice}\nYield: ${JSON.stringify(state.bestYield)}\nSwap: ${state.swapResult}`]
    ]);
    return { ...state, summary: String(response.content) };
  });

  graph.addEdge(START, "wallet");
  graph.addEdge("wallet", "price");
  graph.addEdge("price", "yield");
  graph.addEdge("yield", "swap");
  graph.addEdge("swap", "summarize");
  graph.addEdge("summarize", END);

  return graph.compile();
}
