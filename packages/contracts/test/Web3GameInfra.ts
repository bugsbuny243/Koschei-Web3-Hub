import { expect } from "chai";
import { ethers } from "hardhat";

describe("Web3 Game Infrastructure", function () {
  async function deployAll() {
    const [owner, player1, player2] = await ethers.getSigners();
    const Wallet = await ethers.getContractFactory("CustodialWalletManager");
    const wallet = await Wallet.deploy(owner.address);
    const Asset = await ethers.getContractFactory("GameAsset");
    const asset = await Asset.deploy(owner.address);
    const Profile = await ethers.getContractFactory("PlayerProfile");
    const profile = await Profile.deploy(owner.address);
    return { owner, player1, player2, wallet, asset, profile };
  }

  it("1 create custodial wallet", async () => { const { wallet, player1 } = await deployAll(); await expect(wallet.createCustodialWallet(player1.address, player1.address)).not.to.be.reverted; });
  it("2 reject duplicate wallet", async () => { const { wallet, player1 } = await deployAll(); await wallet.createCustodialWallet(player1.address, player1.address); await expect(wallet.createCustodialWallet(player1.address, player1.address)).to.be.reverted; });
  it("3 pause wallet manager", async () => { const { wallet, player1 } = await deployAll(); await wallet.pause(); await expect(wallet.createCustodialWallet(player1.address, player1.address)).to.be.reverted; });
  it("4 unpause wallet manager", async () => { const { wallet, player1 } = await deployAll(); await wallet.pause(); await wallet.unpause(); await expect(wallet.createCustodialWallet(player1.address, player1.address)).not.to.be.reverted; });
  it("5 fund wallet requires registered", async () => { const { wallet, player1 } = await deployAll(); await expect(wallet.fundWallet(player1.address, { value: 1 })).to.be.reverted; });
  it("6 execute tx requires registered", async () => { const { wallet, player1 } = await deployAll(); await expect(wallet.executeTransaction(player1.address, player1.address, 0, "0x")).to.be.reverted; });

  it("7 mint asset", async () => { const { asset, player1 } = await deployAll(); await expect(asset.mintAsset(player1.address, "hero", "godot-1", "{}")).not.to.be.reverted; });
  it("8 minted token owner", async () => { const { asset, player1 } = await deployAll(); await asset.mintAsset(player1.address, "hero", "godot-1", "{}"); expect(await asset.ownerOf(1)).to.eq(player1.address); });
  it("9 batch mint", async () => { const { asset, player1, player2 } = await deployAll(); await asset.batchMintAssets([player1.address, player2.address], ["a","b"], ["g1","g2"], ["{}","{}"]); expect(await asset.totalSupply()).to.eq(2); });
  it("10 update properties", async () => { const { asset, player1 } = await deployAll(); await asset.mintAsset(player1.address, "hero", "godot-1", "{}"); await asset.updateProperties(1, '{"hp":100}'); expect((await asset.assetData(1)).properties).to.contain("hp"); });
  it("11 enumerable balance", async () => { const { asset, player1 } = await deployAll(); await asset.mintAsset(player1.address, "h", "g", "{}"); expect(await asset.balanceOf(player1.address)).to.eq(1); });
  it("12 only owner mint", async () => { const { asset, player1 } = await deployAll(); await expect(asset.connect(player1).mintAsset(player1.address, "h", "g", "{}")).to.be.reverted; });

  it("13 create profile", async () => { const { profile, player1 } = await deployAll(); await profile.createProfile(player1.address, "alice"); expect((await profile.profiles(player1.address)).exists).to.eq(true); });
  it("14 profile duplicate blocked", async () => { const { profile, player1 } = await deployAll(); await profile.createProfile(player1.address, "alice"); await expect(profile.createProfile(player1.address, "alice")).to.be.reverted; });
  it("15 add xp", async () => { const { profile, player1 } = await deployAll(); await profile.createProfile(player1.address, "alice"); await profile.addExperience(player1.address, 500); expect((await profile.profiles(player1.address)).experience).to.eq(500); });
  it("16 auto level", async () => { const { profile, player1 } = await deployAll(); await profile.createProfile(player1.address, "alice"); await profile.addExperience(player1.address, 1500); expect((await profile.profiles(player1.address)).level).to.eq(2); });
  it("17 multi level", async () => { const { profile, player1 } = await deployAll(); await profile.createProfile(player1.address, "alice"); await profile.addExperience(player1.address, 3500); expect((await profile.profiles(player1.address)).level).to.eq(4); });
  it("18 unlock achievement", async () => { const { profile, player1 } = await deployAll(); await profile.createProfile(player1.address, "alice"); const id = ethers.id("first-blood"); await profile.unlockAchievement(player1.address, id); expect(await profile.achievements(player1.address, id)).to.eq(true); });
  it("19 duplicate achievement blocked", async () => { const { profile, player1 } = await deployAll(); await profile.createProfile(player1.address, "alice"); const id = ethers.id("first-blood"); await profile.unlockAchievement(player1.address, id); await expect(profile.unlockAchievement(player1.address, id)).to.be.reverted; });
  it("20 award tokens", async () => { const { profile, player1 } = await deployAll(); await profile.createProfile(player1.address, "alice"); await profile.awardGameTokens(player1.address, 55); expect((await profile.profiles(player1.address)).gameTokens).to.eq(55); });
  it("21 only owner create profile", async () => { const { profile, player1 } = await deployAll(); await expect(profile.connect(player1).createProfile(player1.address, "alice")).to.be.reverted; });
  it("22 username min length", async () => { const { profile, player1 } = await deployAll(); await expect(profile.createProfile(player1.address, "aa")).to.be.reverted; });
  it("23 zero xp blocked", async () => { const { profile, player1 } = await deployAll(); await profile.createProfile(player1.address, "alice"); await expect(profile.addExperience(player1.address, 0)).to.be.reverted; });
  it("24 zero token blocked", async () => { const { profile, player1 } = await deployAll(); await profile.createProfile(player1.address, "alice"); await expect(profile.awardGameTokens(player1.address, 0)).to.be.reverted; });
  it("25 xp for missing profile blocked", async () => { const { profile, player1 } = await deployAll(); await expect(profile.addExperience(player1.address, 10)).to.be.reverted; });
  it("26 achievements for missing profile blocked", async () => { const { profile, player1 } = await deployAll(); await expect(profile.unlockAchievement(player1.address, ethers.id("x"))).to.be.reverted; });
  it("27 game tokens for missing profile blocked", async () => { const { profile, player1 } = await deployAll(); await expect(profile.awardGameTokens(player1.address, 1)).to.be.reverted; });
  it("28 asset supply starts zero", async () => { const { asset } = await deployAll(); expect(await asset.totalSupply()).to.eq(0); });
  it("29 batch length mismatch blocked", async () => { const { asset, player1 } = await deployAll(); await expect(asset.batchMintAssets([player1.address], ["x","y"], ["g"], ["{}"])) .to.be.reverted; });
  it("30 update missing token blocked", async () => { const { asset } = await deployAll(); await expect(asset.updateProperties(99, "x")).to.be.reverted; });
});
