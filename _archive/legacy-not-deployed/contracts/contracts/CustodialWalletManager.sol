// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

import {Ownable} from "@openzeppelin/contracts/access/Ownable.sol";
import {Pausable} from "@openzeppelin/contracts/utils/Pausable.sol";
import {ReentrancyGuard} from "@openzeppelin/contracts/utils/ReentrancyGuard.sol";

contract CustodialWalletManager is Ownable, Pausable, ReentrancyGuard {
    struct CustodialWallet {
        address player;
        bool exists;
    }

    mapping(address => CustodialWallet) public wallets;

    event CustodialWalletCreated(address indexed player, address indexed walletAddress);
    event WalletFunded(address indexed wallet, uint256 amount);
    event TransactionExecuted(address indexed wallet, address indexed target, uint256 value, bytes data, bytes result);

    constructor(address initialOwner) Ownable(initialOwner) {}

    function createCustodialWallet(address player, address walletAddress) external onlyOwner whenNotPaused {
        require(player != address(0), "invalid player");
        require(walletAddress != address(0), "invalid wallet");
        require(!wallets[walletAddress].exists, "wallet exists");

        wallets[walletAddress] = CustodialWallet({player: player, exists: true});
        emit CustodialWalletCreated(player, walletAddress);
    }

    function fundWallet(address wallet) external payable onlyOwner whenNotPaused nonReentrant {
        require(wallets[wallet].exists, "unknown wallet");
        require(msg.value > 0, "no value");

        (bool sent, ) = wallet.call{value: msg.value}("");
        require(sent, "fund transfer failed");
        emit WalletFunded(wallet, msg.value);
    }

    function executeTransaction(address wallet, address target, uint256 value, bytes calldata data)
        external
        onlyOwner
        whenNotPaused
        nonReentrant
        returns (bytes memory)
    {
        require(wallets[wallet].exists, "unknown wallet");
        require(target != address(0), "invalid target");

        (bool success, bytes memory result) = target.call{value: value}(data);
        require(success, "tx failed");

        emit TransactionExecuted(wallet, target, value, data, result);
        return result;
    }

    function pause() external onlyOwner {
        _pause();
    }

    function unpause() external onlyOwner {
        _unpause();
    }

    receive() external payable {}
}
