import { Wallet } from "ethers";
import { encryptSecret } from "../crypto/encryption";

function main() {
  const wallet = Wallet.createRandom();
  const encryptedPrivateKey = encryptSecret(wallet.privateKey);

  const output = {
    address: wallet.address,
    encryptedPrivateKey,
    chain: "base-sepolia",
    createdAt: new Date().toISOString()
  };

  console.log(JSON.stringify(output, null, 2));
}

main();
