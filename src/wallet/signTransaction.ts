import dotenv from "dotenv";
import { Wallet, parseEther } from "ethers";
import { decryptSecret } from "../crypto/encryption";

dotenv.config();

async function main() {
  const encryptedPrivateKey = process.env.CUSTODIAL_SIGNER_ENCRYPTED_PRIVATE_KEY;
  if (!encryptedPrivateKey) {
    throw new Error("CUSTODIAL_SIGNER_ENCRYPTED_PRIVATE_KEY is missing");
  }

  const privateKey = decryptSecret(encryptedPrivateKey);
  const wallet = new Wallet(privateKey);

  const to = process.env.SAMPLE_TO_ADDRESS;
  if (!to) throw new Error("SAMPLE_TO_ADDRESS is missing");

  const tx = {
    to,
    value: parseEther(process.env.SAMPLE_ETH_AMOUNT || "0.001"),
    nonce: Number(process.env.SAMPLE_NONCE || 0),
    gasLimit: Number(process.env.SAMPLE_GAS_LIMIT || 21000),
    maxFeePerGas: Number(process.env.SAMPLE_MAX_FEE_PER_GAS || 2000000000),
    maxPriorityFeePerGas: Number(process.env.SAMPLE_MAX_PRIORITY_FEE_PER_GAS || 1000000000),
    chainId: Number(process.env.SAMPLE_CHAIN_ID || 84532),
    type: 2
  };

  const signedTransaction = await wallet.signTransaction(tx);
  console.log(signedTransaction);
}

main().catch((error) => {
  console.error(error);
  process.exit(1);
});
