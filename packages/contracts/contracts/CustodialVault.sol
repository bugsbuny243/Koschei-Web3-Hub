// SPDX-License-Identifier: MIT
pragma solidity ^0.8.24;

import "@openzeppelin/contracts/access/Ownable.sol";
import "@openzeppelin/contracts/token/ERC20/IERC20.sol";

contract CustodialVault is Ownable {
    mapping(address => bool) public operators;

    event OperatorUpdated(address indexed operator, bool isAllowed);
    event NativeWithdrawn(address indexed to, uint256 amount);
    event ERC20Withdrawn(address indexed token, address indexed to, uint256 amount);

    modifier onlyOperator() {
        require(operators[msg.sender] || msg.sender == owner(), "NOT_OPERATOR");
        _;
    }

    constructor(address initialOwner) Ownable(initialOwner) {}

    receive() external payable {}

    function setOperator(address operator, bool isAllowed) external onlyOwner {
        require(operator != address(0), "ZERO_ADDRESS");
        operators[operator] = isAllowed;
        emit OperatorUpdated(operator, isAllowed);
    }

    function withdrawNative(address payable to, uint256 amount) external onlyOperator {
        require(to != address(0), "ZERO_ADDRESS");
        require(address(this).balance >= amount, "INSUFFICIENT_BALANCE");

        (bool sent, ) = to.call{value: amount}("");
        require(sent, "NATIVE_TRANSFER_FAILED");

        emit NativeWithdrawn(to, amount);
    }

    function withdrawERC20(address token, address to, uint256 amount) external onlyOperator {
        require(token != address(0) && to != address(0), "ZERO_ADDRESS");
        bool success = IERC20(token).transfer(to, amount);
        require(success, "ERC20_TRANSFER_FAILED");
        emit ERC20Withdrawn(token, to, amount);
    }
}
