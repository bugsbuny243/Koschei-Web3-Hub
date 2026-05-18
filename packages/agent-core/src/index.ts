import "dotenv/config";
import { AutoYieldOptimizerAgent } from "./agents/auto-yield-optimizer-agent";

const demo = async () => {
  const agent = new AutoYieldOptimizerAgent();
  const result = await agent.run({
    prompt: "USDC için en iyi yield stratejisini seç ve kısa özet ver.",
    walletAddress: "0x000000000000000000000000000000000000dead"
  });
  console.log(result);
};

if (process.env.NODE_ENV !== "test") {
  demo().catch(console.error);
}
