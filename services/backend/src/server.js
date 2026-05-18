const express = require('express');
const cors = require('cors');
const rateLimit = require('express-rate-limit');
const Joi = require('joi');
const { ethers } = require('ethers');
const winston = require('winston');
const batcher = require('./batcher');
const { shouldSponsorAddress } = require('./paymaster');
const { createSmartWallet, executeBatchedTransactions } = require('./smartwallet');

const app = express();
app.use(cors());
app.use(express.json());
app.use(rateLimit({ windowMs: 60_000, max: 120 }));

const logger = winston.createLogger({ level: 'info', transports: [new winston.transports.Console()], format: winston.format.combine(winston.format.timestamp(), winston.format.json()) });
const provider = new ethers.JsonRpcProvider(process.env.RPC_URL || 'https://sepolia.base.org');
const signer = new ethers.Wallet(process.env.PRIVATE_KEY || ethers.Wallet.createRandom().privateKey, provider);

const walletCreateSchema = Joi.object({ player: Joi.string().required(), walletAddress: Joi.string().required(), playerEmail: Joi.string().email().required() });
const profileSchema = Joi.object({ player: Joi.string().required(), username: Joi.string().min(3).required(), basename: Joi.string().optional() });
const assetSchema = Joi.object({ to: Joi.string().required(), assetType: Joi.string().required(), godotId: Joi.string().required(), properties: Joi.string().required() });
const xpSchema = Joi.object({ player: Joi.string().required(), amount: Joi.number().integer().positive().required() });
const achievementSchema = Joi.object({ player: Joi.string().required(), achievementId: Joi.string().required() });

const ok = (res, data) => res.json({ success: true, data });

app.post('/api/wallet/create', async (req, res, next) => {
  try {
    const { error, value } = walletCreateSchema.validate(req.body);
    if (error) return next(error);
    const sponsored = await shouldSponsorAddress(value.walletAddress);
    const smartWalletAddress = await createSmartWallet(value.playerEmail);
    logger.info('wallet.create', { ...value, sponsored, smartWalletAddress });
    return ok(res, { txHash: 'pending', sponsored, smartWalletAddress });
  } catch (err) { return next(err); }
});

app.post('/api/profile/create', async (req, res, next) => {
  const { error, value } = profileSchema.validate(req.body);
  if (error) return next(error);
  logger.info('profile.create', value);
  if (value.basename) logger.info('profile.basename.set', { player: value.player, basename: value.basename });
  return ok(res, { txHash: 'pending' });
});

app.post('/api/player/experience', async (req, res, next) => {
  try {
    const { error, value } = xpSchema.validate(req.body);
    if (error) return next(error);
    logger.info('player.experience', value);
    const tx = await playerProfile.addExperience(value.player, value.amount);
    const receipt = await tx.wait();
    return ok(res, { txHash: receipt.hash });
  } catch (err) {
    logger.error('player.experience.error', { message: err.message });
    return res.status(500).json({ success: false, error: err.message });
  }
});

app.post('/api/player/experience', async (req, res, next) => {
  const { error, value } = xpSchema.validate(req.body);
  if (error) return next(error);
  const tx = await batcher.add(async () => ({ txHash: 'pending', player: value.player, amount: value.amount }));
  logger.info('player.experience', value);
  return ok(res, tx);
});

app.post('/api/achievement/unlock', async (req, res, next) => {
  const { error, value } = achievementSchema.validate(req.body);
  if (error) return next(error);
  const result = await executeBatchedTransactions([
    { type: 'unlockAchievement', player: value.player, achievementId: value.achievementId },
    { type: 'mintBadge', player: value.player, achievementId: value.achievementId }
  ]);
  logger.info('achievement.unlock', value);
  return ok(res, result);
});

app.get('/api/metrics', (_req, res) => ok(res, {
  totalPlayers: 0,
  totalAssets: 0,
  dailyActiveUsers: 0,
  weeklyTransactions: 0,
  contractAddresses: {
    achievementBadge: process.env.ACHIEVEMENT_BADGE_ADDRESS || null,
    leaderboard: process.env.LEADERBOARD_ADDRESS || null,
    kosceiMetrics: process.env.KOSCEI_METRICS_ADDRESS || null
  },
  network: 'base-sepolia'
}));

app.get('/api/leaderboard', (_req, res) => ok(res, { topPlayers: [] }));
app.get('/api/player/:address', (req, res) => ok(res, { address: req.params.address, profile: null }));
app.get('/api/assets/:address', (req, res) => ok(res, { address: req.params.address, assets: [] }));

app.use((err, _req, res, _next) => {
  logger.error('api.error', { message: err.message });
  res.status(400).json({ success: false, error: err.message });
});

const port = process.env.PORT || 4000;
if (require.main === module) app.listen(port, () => logger.info(`backend listening on ${port}`));
module.exports = { app, signer, provider };
