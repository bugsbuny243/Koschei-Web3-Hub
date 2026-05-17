// SPDX-License-Identifier: MIT
pragma solidity ^0.8.24;

import "@openzeppelin/contracts/access/Ownable.sol";
import "@openzeppelin/contracts/security/Pausable.sol";
import "@openzeppelin/contracts/security/ReentrancyGuard.sol";
import "@openzeppelin/contracts/token/ERC20/utils/SafeERC20.sol";

contract CustodialVault is Ownable, Pausable, ReentrancyGuard {
    using SafeERC20 for IERC20;

    mapping(address => bool) public operators;
    mapping(address => bool) public allowedRecipients;

    uint256 public dailyNativeWithdrawalLimit;
    uint256 public currentDay;
    uint256 public currentDayNativeWithdrawn;

    error NotOperator();
    error ZeroAddress();
    error ZeroAmount();
    error RecipientNotAllowed(address recipient);
    error DailyLimitExceeded(uint256 attempted, uint256 remaining);
    error NativeTransferFailed();

    event NativeDeposited(address indexed from, uint256 amount);
    event NativeWithdrawn(address indexed to, uint256 amount, address indexed operator);
    event ERC20Withdrawn(address indexed token, address indexed to, uint256 amount, address indexed operator);
    event OperatorUpdated(address indexed operator, bool isAllowed);
    event RecipientUpdated(address indexed recipient, bool isAllowed);
    event DailyLimitUpdated(uint256 newDailyLimit);
    event VaultPaused(address indexed by);
    event VaultUnpaused(address indexed by);

    modifier onlyOperatorOrOwner() {
        if (!(operators[msg.sender] || msg.sender == owner())) revert NotOperator();
        _;
    }

    constructor(address initialOwner, uint256 initialDailyNativeLimit) Ownable(initialOwner) {
        if (initialOwner == address(0)) revert ZeroAddress();
        _setDailyLimit(initialDailyNativeLimit);
    }

    receive() external payable {
        emit NativeDeposited(msg.sender, msg.value);
    }

    function setOperator(address operator, bool isAllowed) external onlyOwner {
        if (operator == address(0)) revert ZeroAddress();
        operators[operator] = isAllowed;
        emit OperatorUpdated(operator, isAllowed);
    }

    function setRecipient(address recipient, bool isAllowed) external onlyOwner {
        if (recipient == address(0)) revert ZeroAddress();
        allowedRecipients[recipient] = isAllowed;
        emit RecipientUpdated(recipient, isAllowed);
    }

    function setDailyNativeWithdrawalLimit(uint256 newDailyLimit) external onlyOwner {
        _setDailyLimit(newDailyLimit);
    }

    function pause() external onlyOwner {
        _pause();
        emit VaultPaused(msg.sender);
    }

    function unpause() external onlyOwner {
        _unpause();
        emit VaultUnpaused(msg.sender);
    }

    function withdrawNative(address payable to, uint256 amount)
        external
        onlyOperatorOrOwner
        whenNotPaused
        nonReentrant
    {
        if (to == address(0)) revert ZeroAddress();
        if (amount == 0) revert ZeroAmount();
        if (!allowedRecipients[to]) revert RecipientNotAllowed(to);

        _rolloverDayIfNeeded();
        uint256 remaining = dailyNativeWithdrawalLimit - currentDayNativeWithdrawn;
        if (amount > remaining) revert DailyLimitExceeded(amount, remaining);

        currentDayNativeWithdrawn += amount;

        (bool sent, ) = to.call{value: amount}("");
        if (!sent) revert NativeTransferFailed();

        emit NativeWithdrawn(to, amount, msg.sender);
    }

    function withdrawERC20(address token, address to, uint256 amount)
        external
        onlyOperatorOrOwner
        whenNotPaused
        nonReentrant
    {
        if (token == address(0) || to == address(0)) revert ZeroAddress();
        if (amount == 0) revert ZeroAmount();
        if (!allowedRecipients[to]) revert RecipientNotAllowed(to);

        IERC20(token).safeTransfer(to, amount);
        emit ERC20Withdrawn(token, to, amount, msg.sender);
    }

    function _setDailyLimit(uint256 newDailyLimit) internal {
        dailyNativeWithdrawalLimit = newDailyLimit;
        emit DailyLimitUpdated(newDailyLimit);
    }

    function _rolloverDayIfNeeded() internal {
        uint256 today = block.timestamp / 1 days;
        if (today != currentDay) {
            currentDay = today;
            currentDayNativeWithdrawn = 0;
        }
    }
}
