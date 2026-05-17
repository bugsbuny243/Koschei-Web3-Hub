import { expect } from "chai";
import { ethers } from "hardhat";

describe("CustodialVault", function () {
  it("lets owner set operator and operator withdraw native asset", async function () {
    const [owner, operator, recipient] = await ethers.getSigners();
    const vault = await ethers.deployContract("CustodialVault", [owner.address]);
    await vault.waitForDeployment();

    await owner.sendTransaction({ to: await vault.getAddress(), value: ethers.parseEther("1") });

    await vault.connect(owner).setOperator(operator.address, true);
    await expect(vault.connect(operator).withdrawNative(recipient.address, ethers.parseEther("0.25")))
      .to.changeEtherBalances([vault, recipient], [ethers.parseEther("-0.25"), ethers.parseEther("0.25")]);
  });
});
