// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

import {Ownable} from "@openzeppelin/contracts/access/Ownable.sol";

contract PlayerProfile is Ownable {
    uint256 public constant XP_PER_LEVEL = 1000;

    struct Profile {
        string username;
        uint256 experience;
        uint256 level;
        uint256 gameTokens;
        bool exists;
    }

    mapping(address => Profile) public profiles;
    mapping(address => mapping(bytes32 => bool)) public achievements;

    event ProfileCreated(address indexed player, string username);
    event ExperienceAdded(address indexed player, uint256 amount, uint256 totalExperience, uint256 newLevel);
    event AchievementUnlocked(address indexed player, bytes32 indexed achievementId);
    event GameTokensAwarded(address indexed player, uint256 amount, uint256 total);

    constructor(address initialOwner) Ownable(initialOwner) {}

    function createProfile(address player, string calldata username) external onlyOwner {
        require(player != address(0), "invalid player");
        require(!profiles[player].exists, "profile exists");
        require(bytes(username).length > 2, "username too short");

        profiles[player] = Profile({username: username, experience: 0, level: 1, gameTokens: 0, exists: true});
        emit ProfileCreated(player, username);
    }

    function addExperience(address player, uint256 amount) external onlyOwner {
        Profile storage profile = profiles[player];
        require(profile.exists, "profile missing");
        require(amount > 0, "amount zero");

        profile.experience += amount;
        uint256 computedLevel = (profile.experience / XP_PER_LEVEL) + 1;
        if (computedLevel > profile.level) {
            profile.level = computedLevel;
        }
        emit ExperienceAdded(player, amount, profile.experience, profile.level);
    }

    function unlockAchievement(address player, bytes32 achievementId) external onlyOwner {
        Profile storage profile = profiles[player];
        require(profile.exists, "profile missing");
        require(!achievements[player][achievementId], "already unlocked");
        achievements[player][achievementId] = true;
        emit AchievementUnlocked(player, achievementId);
    }

    function awardGameTokens(address player, uint256 amount) external onlyOwner {
        Profile storage profile = profiles[player];
        require(profile.exists, "profile missing");
        require(amount > 0, "amount zero");
        profile.gameTokens += amount;
        emit GameTokensAwarded(player, amount, profile.gameTokens);
    }
}
