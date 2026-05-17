import { ethers, network } from "hardhat";

async function main() {
  const [deployer] = await ethers.getSigners();
  const dailyLimit = ethers.parseEther(process.env.INITIAL_NATIVE_DAILY_LIMIT_ETH || "1");

  console.log(`Deployer: ${deployer.address}`);
  console.log(`Network: ${network.name}`);

  const vault = await ethers.deployContract("CustodialVault", [deployer.address, dailyLimit]);
  const deployTx = vault.deploymentTransaction();

  await vault.waitForDeployment();
  const vaultAddress = await vault.getAddress();

  console.log(`Contract: CustodialVault`);
  console.log(`Address: ${vaultAddress}`);
  console.log(`Deploy tx hash: ${deployTx?.hash ?? "N/A"}`);
  console.log(`VAULT_ADDRESS=${vaultAddress}`);
  console.log("Next: npm run verify:base-sepolia -- <VAULT_ADDRESS> <OWNER_ADDRESS> <DAILY_LIMIT_WEI>");
}

main().catch((error) => {
  console.error(error);
  process.exitCode = 1;
});
