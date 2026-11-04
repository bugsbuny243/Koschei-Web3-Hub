package services

import (
	"context"
	"crypto/ed25519"
	"encoding/hex"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"koschei/api/internal/radaradapter/tron"
)

func evidenceFixtureFinding(t *testing.T) TronFlowFinding {
	t.Helper()
	seed, hop, bridge := flowAddr(0x11), flowAddr(0x22), flowAddr(0x55)
	radar, collector := flowRadar(flowConfig(seed), map[string]TronBridge{bridge: {Label: "Fixture Bridge", Source: "https://example.invalid"}}, nil)
	radar.ingest(context.Background(), []tron.BlockTransfer{flowTransfer(1, seed, hop, 500_000, 1000), flowTransfer(2, hop, bridge, 400_000, 1001)})
	exits := collector.ofKind("tron_flow_bridge_exit")
	if len(exits) != 1 {
		t.Fatalf("fixture must produce one bridge exit, got %v", collector.kinds())
	}
	found, ok := radar.FindingByDedupeKey(exits[0].DedupeKey)
	if !ok || found.Kind != "tron_flow_bridge_exit" {
		t.Fatal("finding must be retrievable by dedupe key")
	}
	return found
}

func TestTronFlowEvidencePackageIsSignedAndSurvivesJSONRoundTrip(t *testing.T) {
	seed := make([]byte, ed25519.SeedSize)
	for i := range seed {
		seed[i] = byte(i + 1)
	}
	t.Setenv("KOSCHEI_VERDICT_SIGNING_KEY_ID", "fixture-key-1")
	t.Setenv("KOSCHEI_VERDICT_SIGNING_PRIVATE_KEY", hex.EncodeToString(seed))
	finding := evidenceFixtureFinding(t)

	pkg, err := BuildTronFlowEvidencePackage(finding, time.Date(2026, 11, 4, 9, 0, 0, 0, time.UTC))
	if err != nil || pkg.SignatureStatus != "signed" || pkg.Signature == nil || pkg.Signature.KeyID != "fixture-key-1" {
		t.Fatalf("package must be signed: status=%q err=%v", pkg.SignatureStatus, err)
	}
	if !VerifyTronFlowEvidencePackage(pkg) {
		t.Fatal("a fresh package must verify")
	}
	if strings.Join(pkg.Recipients, "|") != "Fixture Bridge (bridge operator): hold or block request|Tether (USDT issuer): freeze request" {
		t.Fatalf("recipients = %v", pkg.Recipients)
	}
	if !strings.HasPrefix(pkg.PublicLinks["transaction"], "https://tronscan.org/#/transaction/") || pkg.PublicLinks["to_address"] != "https://tronscan.org/#/address/"+tron.Base58(flowAddr(0x55)) {
		t.Fatalf("public links = %v", pkg.PublicLinks)
	}

	// A recipient receives JSON: decoding and re-encoding must keep it verifiable.
	encoded, err := json.Marshal(pkg)
	if err != nil {
		t.Fatal(err)
	}
	var received TronFlowEvidencePackage
	if err := json.Unmarshal(encoded, &received); err != nil {
		t.Fatal(err)
	}
	if !VerifyTronFlowEvidencePackage(received) {
		t.Fatal("a package must still verify after a JSON round trip")
	}
	received.Finding.Message = strings.Replace(received.Finding.Message, "400000", "900000", 1)
	if VerifyTronFlowEvidencePackage(received) {
		t.Fatal("a tampered package must not verify")
	}
	if strings.Contains(string(encoded), hex.EncodeToString(seed)) {
		t.Fatal("the private key must never appear in a package")
	}
}

func TestTronFlowEvidencePackageWithoutSignerSaysSo(t *testing.T) {
	t.Setenv("KOSCHEI_VERDICT_SIGNING_KEY_ID", "")
	t.Setenv("KOSCHEI_VERDICT_SIGNING_PRIVATE_KEY", "")
	pkg, err := BuildTronFlowEvidencePackage(evidenceFixtureFinding(t), time.Now())
	if err != nil || pkg.SignatureStatus != "signer_not_configured" || pkg.Signature != nil || VerifyTronFlowEvidencePackage(pkg) {
		t.Fatalf("an unsigned package must say so and never verify: %+v err=%v", pkg.SignatureStatus, err)
	}
	t.Setenv("KOSCHEI_VERDICT_SIGNING_KEY_ID", "only-id")
	pkg, _ = BuildTronFlowEvidencePackage(evidenceFixtureFinding(t), time.Now())
	if pkg.SignatureStatus != "signer_invalid" || pkg.Signature != nil {
		t.Fatalf("a half-configured signer must be reported, got %q", pkg.SignatureStatus)
	}
}
