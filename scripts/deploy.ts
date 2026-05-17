import { ethers } from "hardhat";

async function main() {
  const [deployer] = await ethers.getSigners();
  console.log(`Deploying with: ${deployer.address}`);

  const vault = await ethers.deployContract("CustodialVault", [deployer.address]);
  await vault.waitForDeployment();

  console.log(`CustodialVault deployed at: ${await vault.getAddress()}`);
}

main().catch((error) => {
  console.error(error);
  process.exitCode = 1;
});
