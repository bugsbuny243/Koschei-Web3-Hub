import { expect } from "chai";
import { ethers } from "hardhat";

describe("CustodialVault", function () {
  async function deployFixture() {
    const [owner, operator, recipient, notAllowedRecipient, other] = await ethers.getSigners();
    const dailyLimit = ethers.parseEther("1");

    const vault = await ethers.deployContract("CustodialVault", [owner.address, dailyLimit]);
    await vault.waitForDeployment();

    const token = await ethers.deployContract("MockERC20");
    await token.waitForDeployment();

    await owner.sendTransaction({ to: await vault.getAddress(), value: ethers.parseEther("2") });
    await token.mint(await vault.getAddress(), ethers.parseEther("100"));

    return { vault, token, owner, operator, recipient, notAllowedRecipient, other, dailyLimit };
  }

  it("deploy olur ve owner doğru atanır", async function () {
    const { vault, owner } = await deployFixture();
    expect(await vault.owner()).to.equal(owner.address);
  });

  it("ETH deposit alınır", async function () {
    const { vault } = await deployFixture();
    expect(await ethers.provider.getBalance(await vault.getAddress())).to.equal(ethers.parseEther("2"));
  });

  it("allowlisted recipient'e withdraw yapılır", async function () {
    const { vault, owner, recipient } = await deployFixture();
    await vault.connect(owner).setRecipient(recipient.address, true);

    await expect(vault.connect(owner).withdrawNative(recipient.address, ethers.parseEther("0.2")))
      .to.changeEtherBalances([vault, recipient], [ethers.parseEther("-0.2"), ethers.parseEther("0.2")]);
  });

  it("allowlist olmayan recipient'e withdraw reddedilir", async function () {
    const { vault, owner, notAllowedRecipient } = await deployFixture();

    await expect(vault.connect(owner).withdrawNative(notAllowedRecipient.address, ethers.parseEther("0.1")))
      .to.be.revertedWithCustomError(vault, "RecipientNotAllowed");
  });

  it("operator yetkisi çalışır", async function () {
    const { vault, owner, operator, recipient } = await deployFixture();
    await vault.connect(owner).setOperator(operator.address, true);
    await vault.connect(owner).setRecipient(recipient.address, true);

    await expect(vault.connect(operator).withdrawNative(recipient.address, ethers.parseEther("0.1")))
      .to.changeEtherBalances([vault, recipient], [ethers.parseEther("-0.1"), ethers.parseEther("0.1")]);
  });

  it("operator olmayan withdraw yapamaz", async function () {
    const { vault, other, recipient, owner } = await deployFixture();
    await vault.connect(owner).setRecipient(recipient.address, true);
    await expect(vault.connect(other).withdrawNative(recipient.address, ethers.parseEther("0.1")))
      .to.be.revertedWithCustomError(vault, "NotOperator");
  });

  it("paused durumda withdraw yapılamaz", async function () {
    const { vault, owner, recipient } = await deployFixture();
    await vault.connect(owner).setRecipient(recipient.address, true);
    await vault.connect(owner).pause();

    await expect(vault.connect(owner).withdrawNative(recipient.address, ethers.parseEther("0.1"))).to.be.revertedWith(
      "Pausable: paused"
    );
  });

  it("unpause sonrası withdraw yapılır", async function () {
    const { vault, owner, recipient } = await deployFixture();
    await vault.connect(owner).setRecipient(recipient.address, true);
    await vault.connect(owner).pause();
    await vault.connect(owner).unpause();

    await expect(vault.connect(owner).withdrawNative(recipient.address, ethers.parseEther("0.1")))
      .to.changeEtherBalances([vault, recipient], [ethers.parseEther("-0.1"), ethers.parseEther("0.1")]);
  });

  it("daily limit aşımı reddedilir", async function () {
    const { vault, owner, recipient, dailyLimit } = await deployFixture();
    await vault.connect(owner).setRecipient(recipient.address, true);

    await expect(vault.connect(owner).withdrawNative(recipient.address, dailyLimit + 1n))
      .to.be.revertedWithCustomError(vault, "DailyLimitExceeded");
  });

  it("ERC20 withdraw çalışır", async function () {
    const { vault, token, owner, recipient } = await deployFixture();
    await vault.connect(owner).setRecipient(recipient.address, true);

    await vault.connect(owner).withdrawERC20(await token.getAddress(), recipient.address, ethers.parseEther("5"));
    expect(await token.balanceOf(recipient.address)).to.equal(ethers.parseEther("5"));
  });

  it("zero amount reddedilir", async function () {
    const { vault, owner, recipient } = await deployFixture();
    await vault.connect(owner).setRecipient(recipient.address, true);

    await expect(vault.connect(owner).withdrawNative(recipient.address, 0)).to.be.revertedWithCustomError(
      vault,
      "ZeroAmount"
    );
  });

  it("zero address reddedilir", async function () {
    const { vault, owner } = await deployFixture();

    await expect(vault.connect(owner).setOperator(ethers.ZeroAddress, true)).to.be.revertedWithCustomError(
      vault,
      "ZeroAddress"
    );
  });
});
