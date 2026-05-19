// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

import {Ownable} from "@openzeppelin/contracts/access/Ownable.sol";

/// @title Koscei Metrics
/// @notice Tracks high-level protocol activity metrics for grant reporting and analytics.
contract KosceiMetrics is Ownable {
    uint256 public totalPlayers;
    uint256 public totalAssetsCreated;
    uint256 public totalExperienceAwarded;
    uint256 public dailyActiveUsers;
    mapping(uint256 => uint256) public dailyTxCount;

    /// @notice Emitted whenever a metric is updated.
    event MetricRecorded(string metricType, uint256 value, uint256 timestamp);

    constructor(address initialOwner) Ownable(initialOwner) {}

    /// @notice Records a newly created player.
    function recordPlayerCreated() external onlyOwner {
        totalPlayers++;
        _recordDailyTx();
        emit MetricRecorded("player_created", totalPlayers, block.timestamp);
    }

    /// @notice Records a newly minted asset.
    function recordAssetMinted() external onlyOwner {
        totalAssetsCreated++;
        _recordDailyTx();
        emit MetricRecorded("asset_minted", totalAssetsCreated, block.timestamp);
    }

    /// @notice Records awarded experience.
    function recordExperienceAwarded(uint256 amount) external onlyOwner {
        totalExperienceAwarded += amount;
        _recordDailyTx();
        emit MetricRecorded("xp_awarded", totalExperienceAwarded, block.timestamp);
    }

    /// @notice Returns transaction count for a given day.
    /// @param dayTimestamp Any timestamp inside the day being queried.
    function getDailyStats(uint256 dayTimestamp) external view returns (uint256) {
        return dailyTxCount[dayTimestamp / 86400];
    }

    function _recordDailyTx() internal {
        dailyTxCount[block.timestamp / 86400]++;
    }
}
