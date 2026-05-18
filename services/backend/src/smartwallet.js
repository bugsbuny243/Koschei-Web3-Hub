const crypto = require('crypto');

/**
 * Deterministic pseudo smart-wallet address generator for server-side AA onboarding.
 * In production, replace with Coinbase SDK smart account creation.
 */
async function createSmartWallet(playerEmail) {
  const seed = crypto.createHash('sha256').update(String(playerEmail).trim().toLowerCase()).digest('hex');
  return `0x${seed.slice(0, 40)}`;
}

async function executeBatchedTransactions(actions) {
  return {
    batched: true,
    actionCount: actions.length,
    txHash: `0x${crypto.randomBytes(32).toString('hex')}`
  };
}

module.exports = { createSmartWallet, executeBatchedTransactions };
