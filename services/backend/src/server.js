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

const {
  PRIVATE_KEY,
  RPC_URL,
  CUSTODIAL_WALLET_MANAGER_ADDRESS,
  GAME_ASSET_ADDRESS,
  PLAYER_PROFILE_ADDRESS
} = process.env;

if (!PRIVATE_KEY) {
  throw new Error('PRIVATE_KEY is required');
}

const provider = new ethers.JsonRpcProvider(RPC_URL || 'https://sepolia.base.org');
const signer = new ethers.Wallet(PRIVATE_KEY, provider);

const CUSTODIAL_ABI = [
  'function createCustodialWallet(address player, address walletAddress)',
  'function executeTransaction(address wallet, address target, uint256 value, bytes data) returns (bytes)'
];
const GAME_ASSET_ABI = [
  'function mintAsset(address to, string assetType, string godotId, string properties) returns (uint256)',
  'event AssetMinted(uint256 indexed tokenId, address indexed to, string assetType, string godotId)'
];
const PLAYER_PROFILE_ABI = [
  'function createProfile(address player, string username)',
  'function addExperience(address player, uint256 amount)'
];

const walletManager = new ethers.Contract(CUSTODIAL_WALLET_MANAGER_ADDRESS, CUSTODIAL_ABI, signer);
const gameAsset = new ethers.Contract(GAME_ASSET_ADDRESS, GAME_ASSET_ABI, signer);
const playerProfile = new ethers.Contract(PLAYER_PROFILE_ADDRESS, PLAYER_PROFILE_ABI, signer);

const ethAddress = Joi.string().pattern(/^0x[a-fA-F0-9]{40}$/).required();
const walletCreateSchema = Joi.object({ player: ethAddress, walletAddress: ethAddress });
const profileSchema = Joi.object({ player: ethAddress, username: Joi.string().min(3).required() });
const assetSchema = Joi.object({
  to: ethAddress,
  assetType: Joi.string().required(),
  godotId: Joi.string().required(),
  properties: Joi.string().required()
});
const xpSchema = Joi.object({ player: ethAddress, amount: Joi.number().integer().positive().required() });

const ok = (res, data) => res.json({ success: true, data });

app.post('/api/wallet/create', async (req, res, next) => {
  try {
    const { error, value } = walletCreateSchema.validate(req.body);
    if (error) return next(error);
    logger.info('wallet.create', value);
    const tx = await walletManager.createCustodialWallet(value.player, value.walletAddress);
    const receipt = await tx.wait();
    return ok(res, { txHash: receipt.hash });
  } catch (err) {
    logger.error('wallet.create.error', { message: err.message });
    return res.status(500).json({ success: false, error: err.message });
  }
});

app.post('/api/profile/create', async (req, res, next) => {
  try {
    const { error, value } = profileSchema.validate(req.body);
    if (error) return next(error);
    logger.info('profile.create', value);
    const tx = await playerProfile.createProfile(value.player, value.username);
    const receipt = await tx.wait();
    return ok(res, { txHash: receipt.hash });
  } catch (err) {
    logger.error('profile.create.error', { message: err.message });
    return res.status(500).json({ success: false, error: err.message });
  }
});

app.post('/api/asset/mint', async (req, res, next) => {
  try {
    const { error, value } = assetSchema.validate(req.body);
    if (error) return next(error);
    logger.info('asset.mint', value);
    const tx = await gameAsset.mintAsset(value.to, value.assetType, value.godotId, value.properties);
    const receipt = await tx.wait();
    const iface = gameAsset.interface;
    let tokenId = null;
    for (const log of receipt.logs) {
      try {
        const parsed = iface.parseLog(log);
        if (parsed && parsed.name === 'AssetMinted') {
          tokenId = parsed.args.tokenId.toString();
          break;
        }
      } catch (_err) {
        // ignore non-matching logs
      }
    }
    return ok(res, { tokenId, txHash: receipt.hash });
  } catch (err) {
    logger.error('asset.mint.error', { message: err.message });
    return res.status(500).json({ success: false, error: err.message });
  }
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

app.get('/health', async (_req, res) => {
  try {
    const network = await provider.getNetwork();
    return ok(res, {
      status: 'ok',
      signerAddress: signer.address,
      network: { chainId: network.chainId.toString(), name: network.name }
    });
  } catch (err) {
    logger.error('health.error', { message: err.message });
    return res.status(500).json({ success: false, error: err.message });
  }
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
