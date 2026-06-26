import { ArvisClient } from "../../sdk/typescript/src/index.js";

const mint = process.argv[2];
if (!mint) {
  throw new Error("Usage: node index.js <token-mint>");
}

const apiKey = process.env.ARVIS_API_KEY;
if (!apiKey) {
  throw new Error("ARVIS_API_KEY is required");
}

const arvis = new ArvisClient({ apiKey });
const queued = await arvis.tokenScan({
  mint,
  network: "solana-mainnet",
  include_ai: false
});

console.log({
  decision: "pending_analysis",
  mint,
  requestId: queued.request_id,
  status: queued.status,
  costCredits: queued.cost_credits,
  nextStep: "Read the usage endpoint or partner job surface before listing the token."
});
