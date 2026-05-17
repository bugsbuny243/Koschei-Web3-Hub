const express = require('express');
const cors = require('cors');
const rateLimit = require('express-rate-limit');
const Joi = require('joi');
const { ethers } = require('ethers');
const winston = require('winston');

const app = express();
app.use(cors());
app.use(express.json());
app.use(rateLimit({ windowMs: 60_000, max: 120 }));

const logger = winston.createLogger({
  level: 'info',
  transports: [new winston.transports.Console()],
  format: winston.format.combine(winston.format.timestamp(), winston.format.json())
});

const provider = new ethers.JsonRpcProvider(process.env.RPC_URL || 'https://sepolia.base.org');
const signer = new ethers.Wallet(process.env.PRIVATE_KEY || ethers.Wallet.createRandom().privateKey, provider);

const walletCreateSchema = Joi.object({ player: Joi.string().required(), walletAddress: Joi.string().required() });
const profileSchema = Joi.object({ player: Joi.string().required(), username: Joi.string().min(3).required() });
const assetSchema = Joi.object({ to: Joi.string().required(), assetType: Joi.string().required(), godotId: Joi.string().required(), properties: Joi.string().required() });
const xpSchema = Joi.object({ player: Joi.string().required(), amount: Joi.number().integer().positive().required() });

const ok = (res, data) => res.json({ success: true, data });

app.post('/api/wallet/create', (req, res, next) => {
  const { error, value } = walletCreateSchema.validate(req.body);
  if (error) return next(error);
  logger.info('wallet.create', value);
  return ok(res, { txHash: 'pending' });
});

app.post('/api/profile/create', (req, res, next) => {
  const { error, value } = profileSchema.validate(req.body);
  if (error) return next(error);
  logger.info('profile.create', value);
  return ok(res, { txHash: 'pending' });
});

app.post('/api/asset/mint', (req, res, next) => {
  const { error, value } = assetSchema.validate(req.body);
  if (error) return next(error);
  logger.info('asset.mint', value);
  return ok(res, { txHash: 'pending' });
});

app.post('/api/player/experience', (req, res, next) => {
  const { error, value } = xpSchema.validate(req.body);
  if (error) return next(error);
  logger.info('player.experience', value);
  return ok(res, { txHash: 'pending' });
});

app.get('/api/player/:address', (req, res) => ok(res, { address: req.params.address, profile: null }));
app.get('/api/assets/:address', (req, res) => ok(res, { address: req.params.address, assets: [] }));

app.use((err, _req, res, _next) => {
  logger.error('api.error', { message: err.message });
  res.status(400).json({ success: false, error: err.message });
});

const port = process.env.PORT || 4000;
if (require.main === module) {
  app.listen(port, () => logger.info(`backend listening on ${port}`));
}

module.exports = { app, signer, provider };
