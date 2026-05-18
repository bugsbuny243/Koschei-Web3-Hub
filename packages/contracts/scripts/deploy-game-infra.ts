import { ethers, run } from "hardhat";
import fs from "node:fs";

async function verify(address: string, args: unknown[] = []) {
  try { await run("verify:verify", { address, constructorArguments: args }); } catch {}
}

async function main() {
  const [deployer] = await ethers.getSigners();
  const network = "base-sepolia";
  const scanBase = "https://sepolia.basescan.org/address";

  const wallet = await (await ethers.getContractFactory("CustodialWalletManager")).deploy(deployer.address); await wallet.waitForDeployment();
  const asset = await (await ethers.getContractFactory("GameAsset")).deploy(deployer.address); await asset.waitForDeployment();
  const profile = await (await ethers.getContractFactory("PlayerProfile")).deploy(deployer.address); await profile.waitForDeployment();
  const vault = await (await ethers.getContractFactory("CustodialVault")).deploy(deployer.address); await vault.waitForDeployment();
  const badge = await (await ethers.getContractFactory("AchievementBadge")).deploy(deployer.address); await badge.waitForDeployment();
  const leaderboard = await (await ethers.getContractFactory("Leaderboard")).deploy(deployer.address); await leaderboard.waitForDeployment();
  const metrics = await (await ethers.getContractFactory("KosceiMetrics")).deploy(deployer.address); await metrics.waitForDeployment();

  const addresses = {
    network,
    custodialWalletManager: await wallet.getAddress(),
    gameAsset: await asset.getAddress(),
    playerProfile: await profile.getAddress(),
    custodialVault: await vault.getAddress(),
    achievementBadge: await badge.getAddress(),
    leaderboard: await leaderboard.getAddress(),
    kosceiMetrics: await metrics.getAddress(),
  };

  fs.writeFileSync("packages/contracts/deployments/base-sepolia.json", JSON.stringify(addresses, null, 2));

  await Promise.all([
    verify(addresses.custodialWalletManager, [deployer.address]),
    verify(addresses.gameAsset, [deployer.address]),
    verify(addresses.playerProfile, [deployer.address]),
    verify(addresses.custodialVault, [deployer.address]),
    verify(addresses.achievementBadge, [deployer.address]),
    verify(addresses.leaderboard, [deployer.address]),
    verify(addresses.kosceiMetrics, [deployer.address]),
  ]);

  console.log("Deployment summary:");
  for (const [name, address] of Object.entries(addresses)) {
    if (name === "network") continue;
    console.log(`${name}: ${address} (${scanBase}/${address})`);
  }
}

main().catch((error) => { console.error(error); process.exitCode = 1; });
