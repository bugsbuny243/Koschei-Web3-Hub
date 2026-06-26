import { ArvisClient, isSignedVerdict } from "../../sdk/typescript/src/index.js";

const target = process.argv[2];
if (!target) {
  throw new Error("Usage: node index.js <solana-target>");
}

const apiKey = process.env.ARVIS_API_KEY;
if (!apiKey) {
  throw new Error("ARVIS_API_KEY is required");
}

const arvis = new ArvisClient({ apiKey });
const result = await arvis.shieldPreflight({
  target,
  context: { surface: "wallet_warning_example" }
});

if (!isSignedVerdict(result)) {
  console.log({ action: "withhold", message: "Signed verdict unavailable" });
  process.exit(0);
}

const action = String(result.action ?? "allow");
const message =
  action === "block"
    ? "Block this interaction"
    : action === "warn"
      ? "Show a high-visibility warning"
      : action === "allow_with_monitoring"
        ? "Allow with monitoring"
        : "Allow";

console.log({
  target,
  action,
  message,
  grade: result.grade,
  riskIndex: result.risk_index,
  riskLevel: result.risk_level,
  verdict: result.verdict,
  recommendation: result.recommendation
});
