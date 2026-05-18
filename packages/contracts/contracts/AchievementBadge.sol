// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

import {ERC1155} from "@openzeppelin/contracts/token/ERC1155/ERC1155.sol";
import {Ownable} from "@openzeppelin/contracts/access/Ownable.sol";

/// @title Achievement Badge
/// @notice ERC-1155 badge contract to mint on-chain achievement NFTs for players.
contract AchievementBadge is ERC1155, Ownable {
    mapping(bytes32 => uint256) public achievementToTokenId;
    mapping(uint256 => string) public tokenMetadata;
    uint256 private _nextTokenId;

    event BadgeMinted(address indexed player, bytes32 indexed achievementId, uint256 tokenId);

    constructor(address initialOwner) ERC1155("https://koscei.io/api/badge/{id}") Ownable(initialOwner) {}

    /// @notice Creates a badge type for an achievement identifier.
    function createBadgeType(bytes32 achievementId, string calldata metadata) external onlyOwner returns (uint256) {
        uint256 tokenId = ++_nextTokenId;
        achievementToTokenId[achievementId] = tokenId;
        tokenMetadata[tokenId] = metadata;
        return tokenId;
    }

    /// @notice Mints a badge NFT to player for an existing achievement type.
    function mintBadge(address player, bytes32 achievementId) external onlyOwner {
        uint256 tokenId = achievementToTokenId[achievementId];
        require(tokenId > 0, "badge type not found");
        _mint(player, tokenId, 1, "");
        emit BadgeMinted(player, achievementId, tokenId);
    }
}
