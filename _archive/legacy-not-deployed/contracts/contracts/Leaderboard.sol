// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

import {Ownable} from "@openzeppelin/contracts/access/Ownable.sol";

/// @title On-chain Leaderboard
/// @notice Maintains top 10 players by score for transparent game ranking.
contract Leaderboard is Ownable {
    struct Entry {
        address player;
        string username;
        uint256 score;
        uint256 updatedAt;
    }

    Entry[10] public topPlayers;
    mapping(address => uint256) public playerScores;

    event LeaderboardUpdated(address indexed player, uint256 score, uint256 rank);

    constructor(address initialOwner) Ownable(initialOwner) {}

    /// @notice Updates player score and potentially top 10 placement.
    function updateScore(address player, string calldata username, uint256 score) external onlyOwner {
        playerScores[player] = score;
        _updateTopPlayers(player, username, score);
    }

    function _updateTopPlayers(address player, string memory username, uint256 score) internal {
        if (score <= topPlayers[9].score) return;

        topPlayers[9] = Entry(player, username, score, block.timestamp);

        for (uint256 i = 9; i > 0; i--) {
            if (topPlayers[i].score > topPlayers[i - 1].score) {
                Entry memory temp = topPlayers[i];
                topPlayers[i] = topPlayers[i - 1];
                topPlayers[i - 1] = temp;
                emit LeaderboardUpdated(player, score, i);
            } else {
                break;
            }
        }
    }

    /// @notice Returns full top 10 entries.
    function getTopPlayers() external view returns (Entry[10] memory) {
        return topPlayers;
    }
}
