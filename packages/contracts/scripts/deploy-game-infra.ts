import { ethers } from "hardhat";

async function main() {
  const [deployer] = await ethers.getSigners();
  console.log("Deploying with:", deployer.address);

  const wallet = await (await ethers.getContractFactory("CustodialWalletManager")).deploy(deployer.address);
  await wallet.waitForDeployment();

  const asset = await (await ethers.getContractFactory("GameAsset")).deploy(deployer.address);
  await asset.waitForDeployment();

  const profile = await (await ethers.getContractFactory("PlayerProfile")).deploy(deployer.address);
  await profile.waitForDeployment();

  console.log("CustodialWalletManager:", await wallet.getAddress());
  console.log("GameAsset:", await asset.getAddress());
  console.log("PlayerProfile:", await profile.getAddress());
}

main().catch((error) => {
  console.error(error);
  process.exitCode = 1;
});
