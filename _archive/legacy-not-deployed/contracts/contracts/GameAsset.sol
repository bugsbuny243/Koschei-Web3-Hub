// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

import {Ownable} from "@openzeppelin/contracts/access/Ownable.sol";
import {ERC721Enumerable} from "@openzeppelin/contracts/token/ERC721/extensions/ERC721Enumerable.sol";

contract GameAsset is ERC721Enumerable, Ownable {
    struct AssetData {
        string assetType;
        string godotId;
        string properties;
        uint256 mintedAt;
    }

    uint256 private _nextTokenId;
    mapping(uint256 => AssetData) public assetData;
    mapping(string => bool) public godotIdUsed;

    event AssetMinted(uint256 indexed tokenId, address indexed to, string assetType, string godotId);
    event AssetPropertiesUpdated(uint256 indexed tokenId, string properties);

    constructor(address initialOwner) ERC721("KosceiGameAsset", "KGA") Ownable(initialOwner) {}

    function mintAsset(address to, string calldata assetType, string calldata godotId, string calldata properties)
        external
        onlyOwner
        returns (uint256)
    {
        require(to != address(0), "invalid recipient");
        require(!godotIdUsed[godotId], "godotId already minted");
        godotIdUsed[godotId] = true;

        uint256 tokenId = ++_nextTokenId;
        _safeMint(to, tokenId);
        assetData[tokenId] = AssetData(assetType, godotId, properties, block.timestamp);
        emit AssetMinted(tokenId, to, assetType, godotId);
        return tokenId;
    }

    function batchMintAssets(
        address[] calldata recipients,
        string[] calldata assetTypes,
        string[] calldata godotIds,
        string[] calldata propertiesList
    ) external onlyOwner {
        uint256 length = recipients.length;
        require(length > 0, "empty batch");
        require(length == assetTypes.length && length == godotIds.length && length == propertiesList.length, "length mismatch");

        for (uint256 i = 0; i < length; i++) {
            mintAsset(recipients[i], assetTypes[i], godotIds[i], propertiesList[i]);
        }
    }

    function updateProperties(uint256 tokenId, string calldata properties) external onlyOwner {
        require(_ownerOf(tokenId) != address(0), "missing token");
        assetData[tokenId].properties = properties;
        emit AssetPropertiesUpdated(tokenId, properties);
    }
}
