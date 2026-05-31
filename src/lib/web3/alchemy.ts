import "server-only";

type Chain = "solana" | "base" | "arbitrum" | "polygon" | "optimism" | "ethereum";
type HealthStatus = "online" | "error" | "not-configured";
type Config = { network: string; env: string; alchemy: (key: string) => string; method: string };

const configs: Record<Chain, Config> = {
  solana: { network: process.env.SOLANA_NETWORK || process.env.NEXT_PUBLIC_SOLANA_NETWORK || "devnet", env: "SOLANA_RPC_URL", alchemy: (key) => `https://solana-devnet.g.alchemy.com/v2/${key}`, method: "getHealth" },
  base: { network: process.env.BASE_NETWORK || process.env.NEXT_PUBLIC_BASE_NETWORK || "sepolia", env: "BASE_RPC_URL", alchemy: (key) => `https://base-sepolia.g.alchemy.com/v2/${key}`, method: "eth_blockNumber" },
  arbitrum: { network: process.env.ARBITRUM_NETWORK || process.env.NEXT_PUBLIC_ARBITRUM_NETWORK || "sepolia", env: "ARBITRUM_RPC_URL", alchemy: (key) => `https://arb-sepolia.g.alchemy.com/v2/${key}`, method: "eth_blockNumber" },
  polygon: { network: process.env.POLYGON_NETWORK || process.env.NEXT_PUBLIC_POLYGON_NETWORK || "amoy", env: "POLYGON_RPC_URL", alchemy: (key) => `https://polygon-amoy.g.alchemy.com/v2/${key}`, method: "eth_blockNumber" },
  optimism: { network: process.env.OPTIMISM_NETWORK || process.env.NEXT_PUBLIC_OPTIMISM_NETWORK || "sepolia", env: "OPTIMISM_RPC_URL", alchemy: (key) => `https://opt-sepolia.g.alchemy.com/v2/${key}`, method: "eth_blockNumber" },
  ethereum: { network: process.env.ETHEREUM_NETWORK || process.env.NEXT_PUBLIC_ETHEREUM_NETWORK || "sepolia", env: "ETHEREUM_RPC_URL", alchemy: (key) => `https://eth-sepolia.g.alchemy.com/v2/${key}`, method: "eth_blockNumber" },
};

export function isSupportedChain(value: string): value is Chain {
  return value in configs;
}

function readableError(error: unknown, rpc: string, key?: string) {
  const fallback = "RPC request failed.";
  if (!(error instanceof Error)) return fallback;
  let message = error.message || fallback;
  if (rpc) message = message.replaceAll(rpc, "the configured provider");
  if (key) message = message.replaceAll(key, "[redacted]");
  return message;
}

export async function checkChainHealth(chain: Chain) {
  const config = configs[chain];
  const explicit = process.env[config.env]?.trim();
  const key = process.env.ALCHEMY_API_KEY?.trim();
  const rpc = explicit || (key ? config.alchemy(key) : "");
  const provider = explicit ? "Custom server-side RPC" : "Alchemy";

  if (!rpc) {
    return { ok: false, status: "not-configured" as HealthStatus, chain, network: config.network, provider, error: "RPC provider is not configured on the server." };
  }

  try {
    const response = await fetch(rpc, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ jsonrpc: "2.0", id: 1, method: config.method, params: [] }),
      signal: AbortSignal.timeout(10_000),
      cache: "no-store",
    });
    if (!response.ok) throw new Error(`Provider returned HTTP ${response.status}.`);
    const json = await response.json() as { result?: unknown; error?: { message?: string } };
    if (json.error) throw new Error(json.error.message || "RPC request failed.");
    return { ok: true, status: "online" as HealthStatus, chain, network: config.network, provider };
  } catch (error) {
    return { ok: false, status: "error" as HealthStatus, chain, network: config.network, provider, error: readableError(error, rpc, key) };
  }
}
