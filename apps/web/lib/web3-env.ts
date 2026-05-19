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
const optionalVarSet = new Set<string>(optionalVars);

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
    const isBuildPhase = process.env.NEXT_PHASE === "phase-production-build";
    if (!isBuildPhase) {
      throw new Error(`Missing required environment variables: ${missing.join(", ")}`);
    }

    for (const key of missing) {
      env[key as keyof RequiredEnv] = "";
    }
  }

  return env;
}

let cachedEnv: (RequiredEnv & OptionalEnv) | null = null;

function resolveEnv() {
  if (!cachedEnv) {
    cachedEnv = getEnv();
  }
  return cachedEnv;
}

export const web3Env: RequiredEnv & OptionalEnv = new Proxy({} as RequiredEnv & OptionalEnv, {
  get(_target, prop: string) {
    if (optionalVarSet.has(prop)) {
      return process.env[prop];
    }
    return resolveEnv()[prop as keyof (RequiredEnv & OptionalEnv)];
  }
});
