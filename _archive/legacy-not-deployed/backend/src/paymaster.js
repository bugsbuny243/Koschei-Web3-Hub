const { createPublicClient, createWalletClient, http } = require('viem');
const { privateKeyToAccount } = require('viem/accounts');
const { baseSepolia } = require('viem/chains');

const paymasterUrl = process.env.PAYMASTER_URL || `https://api.developer.coinbase.com/rpc/v1/base-sepolia/${process.env.CDP_API_KEY || ''}`;
const rpcUrl = process.env.RPC_URL || 'https://sepolia.base.org';

const publicClient = createPublicClient({ chain: baseSepolia, transport: http(rpcUrl) });

function createSponsoredClient(privateKey) {
  const account = privateKeyToAccount(privateKey);
  const walletClient = createWalletClient({ account, chain: baseSepolia, transport: http(paymasterUrl) });
  return { account, walletClient };
}

async function shouldSponsorAddress(address) {
  const txCount = await publicClient.getTransactionCount({ address });
  return txCount === 0n;
}

module.exports = {
  createSponsoredClient,
  shouldSponsorAddress,
  paymasterUrl
};
