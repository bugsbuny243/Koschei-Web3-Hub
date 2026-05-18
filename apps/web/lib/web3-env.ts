const requiredVars = [
  "DATABASE_URL",
  "ALCHEMY_API_KEY",
  "ARBITRUM_RPC_URL",
  "ARBITRUM_SEPOLIA_RPC_URL",
  "WEBHOOK_SECRET",
  "CRON_SECRET"
] as const;

const optionalVars = [
  "ALCHEMY_WEBHOOK_SIGNING_KEY",
  "NEON_AUTH_BASE_URL",
  "NEON_AUTH_COOKIE_SECRET"
] as const;

type RequiredEnv = { [K in (typeof requiredVars)[number]]: string };
type OptionalEnv = { [K in (typeof optionalVars)[number]]?: string };

function getEnv() {
  const missing: string[] = [];
  const env = {} as RequiredEnv & OptionalEnv;

  for (const key of requiredVars) {
    const value = process.env[key];
    if (!value) {
      missing.push(key);
      continue;
    }
    env[key] = value;
  }

  for (const key of optionalVars) {
    const value = process.env[key];
    if (value) {
      env[key] = value;
    }
  }

  if (missing.length > 0) {
    throw new Error(`Missing required environment variables: ${missing.join(", ")}`);
  }

  return env;
}

export const web3Env = getEnv();
