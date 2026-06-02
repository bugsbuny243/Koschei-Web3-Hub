package handlers

import (
	"encoding/json"
	"testing"
)

func TestBuildGeneratedMetadata(t *testing.T) {
	metadata := buildGeneratedMetadata(
		"Cosmic Dragon #001",
		"A legendary cyber dragon NFT...",
		"NFT",
		" fire, legendary, , dragon, neon, armor, 5-star ",
	)

	if metadata.Name != "Cosmic Dragon #001" {
		t.Fatalf("Name = %q, want Cosmic Dragon #001", metadata.Name)
	}
	if metadata.Type != "NFT" {
		t.Fatalf("Type = %q, want NFT", metadata.Type)
	}
	if metadata.Properties.Category != "game_asset" {
		t.Fatalf("Category = %q, want game_asset", metadata.Properties.Category)
	}
	if metadata.Properties.GeneratedBy != "Koschei Web3 Hub" {
		t.Fatalf("GeneratedBy = %q, want Koschei Web3 Hub", metadata.Properties.GeneratedBy)
	}

	wantTraits := []string{"fire", "legendary", "dragon", "neon", "armor", "5-star"}
	if len(metadata.Attributes) != len(wantTraits) {
		t.Fatalf("len(Attributes) = %d, want %d", len(metadata.Attributes), len(wantTraits))
	}
	for i, want := range wantTraits {
		if metadata.Attributes[i].TraitType != "Trait" || metadata.Attributes[i].Value != want {
			t.Fatalf("Attributes[%d] = %#v, want Trait/%q", i, metadata.Attributes[i], want)
		}
	}

	if _, err := json.Marshal(metadata); err != nil {
		t.Fatalf("json.Marshal(metadata) error = %v", err)
	}
}

func TestMetadataAttributesEmptyTraitsReturnsEmptyArray(t *testing.T) {
	attributes := metadataAttributes(" ,  , ")
	if attributes == nil {
		t.Fatal("metadataAttributes() = nil, want an empty JSON array")
	}
	if len(attributes) != 0 {
		t.Fatalf("len(metadataAttributes()) = %d, want 0", len(attributes))
	}
}
