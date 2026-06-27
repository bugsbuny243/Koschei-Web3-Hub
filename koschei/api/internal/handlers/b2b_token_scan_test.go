package handlers

import (
	"reflect"
	"testing"
)

func TestNormalizeB2BTokenTargetsSingle(t *testing.T) {
	got := normalizeB2BTokenTargets(b2bTokenScanRequest{Mint: "  TokenMint  "})
	want := []string{"TokenMint"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("normalizeB2BTokenTargets() = %#v, want %#v", got, want)
	}
}

func TestNormalizeB2BTokenTargetsBatchDeduplicates(t *testing.T) {
	got := normalizeB2BTokenTargets(b2bTokenScanRequest{Mints: []string{"MintA", " MintB ", "MintA", ""}})
	want := []string{"MintA", "MintB"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("normalizeB2BTokenTargets() = %#v, want %#v", got, want)
	}
}

func TestNormalizeB2BTokenTargetsPreservesBase58Case(t *testing.T) {
	got := normalizeB2BTokenTargets(b2bTokenScanRequest{Mints: []string{"AbC", "aBc"}})
	want := []string{"AbC", "aBc"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("normalizeB2BTokenTargets() = %#v, want %#v", got, want)
	}
}
