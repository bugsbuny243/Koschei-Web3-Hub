import dotenv from "dotenv";
import { ethers } from "hardhat";

dotenv.config();

async function main() {
  const vaultAddress = process.env.VAULT_ADDRESS;
  if (!vaultAddress) throw new Error("VAULT_ADDRESS is missing");

  const [signer] = await ethers.getSigners();
  const vault = await ethers.getContractAt("CustodialVault", vaultAddress, signer);

  const operatorAddress = process.env.OPERATOR_ADDRESS;
  if (operatorAddress) {
    const allowOperator = (process.env.ALLOW_OPERATOR || "true").toLowerCase() === "true";
    const tx = await vault.setOperator(operatorAddress, allowOperator);
    await tx.wait();
    console.log(`Operator updated: ${operatorAddress} => ${allowOperator}, tx: ${tx.hash}`);
  }

  const recipientAddress = process.env.RECIPIENT_ADDRESS;
  if (recipientAddress) {
    const allowRecipient = (process.env.ALLOW_RECIPIENT || "true").toLowerCase() === "true";
    const tx = await vault.setRecipient(recipientAddress, allowRecipient);
    await tx.wait();
    console.log(`Recipient updated: ${recipientAddress} => ${allowRecipient}, tx: ${tx.hash}`);
  }

  const dailyLimitEth = process.env.INITIAL_NATIVE_DAILY_LIMIT_ETH;
  if (dailyLimitEth) {
    const tx = await vault.setDailyNativeWithdrawalLimit(ethers.parseEther(dailyLimitEth));
    await tx.wait();
    console.log(`Daily native limit updated: ${dailyLimitEth} ETH, tx: ${tx.hash}`);
  }
}

main().catch((error) => {
  console.error(error);
  process.exitCode = 1;
});
