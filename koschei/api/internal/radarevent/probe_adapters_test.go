package radarevent

import (
	"strings"
	"testing"
	"time"

	"koschei/api/internal/networktarget"
	"koschei/api/internal/securityevidence"
)

func TestBuildEVMAddressProbeEventKeepsAccountEvidenceChainIndependent(t *testing.T) {
	resolution, err := networktarget.Resolve("ethereum-mainnet", "0x1111111111111111111111111111111111111111")
	if err != nil {
		t.Fatal(err)
	}
	result := networktarget.EVMProbeResult{
		SchemaVersion:     networktarget.SchemaVersion,
		Resolution:        resolution,
		ChainID:           "0x1",
		ExpectedChainID:   "0x1",
		ContractCodeState: "contract_code_observed",
		ContractCodeHash:  strings.Repeat("a", 64),
		DelegationState:   networktarget.EVMDelegationStateNotObserved,
		AnalysisPerformed: true,
		EvidenceStatus:    "observed",
		LiveAvailability:  "checked",
	}

	sourceDigest := strings.Repeat("b", 64)
	event, err := BuildEVMAddressProbeEvent("evm-rpc-adapter", result, time.UnixMilli(1780000000000), sourceDigest)
	if err != nil {
		t.Fatal(err)
	}
	if event.Kind != KindAccount || event.NetworkID != "ethereum-mainnet" || event.State != securityevidence.StateObserved {
		t.Fatalf("unexpected event: %#v", event)
	}
	if err := event.Verify(); err != nil {
		t.Fatal(err)
	}
	for _, fact := range event.Facts {
		if fact.EvidenceSHA256 != sourceDigest {
			t.Fatalf("fact %q lost source binding", fact.Key)
		}
	}
}

func TestBuildBitcoinAddressProbeEventPreservesUTXOActivityWithoutSafetyClaim(t *testing.T) {
	resolution, err := networktarget.Resolve("bitcoin-mainnet", "1BoatSLRHtKNngkdXEeobR76b53LETtpyT")
	if err != nil {
		t.Fatal(err)
	}
	result := networktarget.BitcoinProbeResult{
		SchemaVersion:       networktarget.SchemaVersion,
		Resolution:          resolution,
		GenesisHash:         strings.Repeat("0", 64),
		ExpectedGenesisHash: strings.Repeat("0", 64),
		ActivityState:       "activity_observed",
		ConfirmedTXCount:    12,
		MempoolTXCount:      1,
		FundedSats:          42000,
		SpentSats:           21000,
		AnalysisPerformed:   true,
		EvidenceStatus:      "observed",
		LiveAvailability:    "checked",
	}

	event, err := BuildBitcoinAddressProbeEvent("bitcoin-esplora-adapter", result, time.UnixMilli(1780000000000), strings.Repeat("c", 64))
	if err != nil {
		t.Fatal(err)
	}
	if event.Kind != KindAccount || event.SubjectKind != "address" {
		t.Fatalf("unexpected event identity: %#v", event)
	}
	if err := event.Verify(); err != nil {
		t.Fatal(err)
	}
	for _, fact := range event.Facts {
		if fact.Key == "safe" || fact.Key == "risk" {
			t.Fatalf("adapter minted a decision fact: %#v", fact)
		}
	}
}

func TestBuildMoveIdentityEventsPreserveVerifiedChainIdentity(t *testing.T) {
	suiObservation, err := networktarget.NormalizeNetworkTelemetry(networktarget.NetworkTelemetryInput{
		NetworkID:      "sui-mainnet",
		SubjectKind:    "network",
		SubjectID:      "sui-mainnet",
		Source:         "sui-graphql-chain-identity:node.example",
		ObservedAt:     time.UnixMilli(1780000000000),
		EvidenceStatus: "verified",
	})
	if err != nil {
		t.Fatal(err)
	}
	sui, err := BuildSuiIdentityEvent("sui-identity-adapter", networktarget.SuiMainnetIdentityProbeResult{
		SchemaVersion:     networktarget.NetworkTelemetrySchemaVersion,
		Observation:       suiObservation,
		ChainIdentifier:   "mainnet-identifier",
		EndpointScope:     "sui_graphql_chain_identity_only",
		AnalysisPerformed: true,
		LiveAvailability:  "checked",
	}, strings.Repeat("d", 64))
	if err != nil {
		t.Fatal(err)
	}
	if sui.State != securityevidence.StateVerified || sui.Kind != KindNetworkHealth {
		t.Fatalf("unexpected sui event: %#v", sui)
	}
	if err := sui.Verify(); err != nil {
		t.Fatal(err)
	}

	aptosObservation, err := networktarget.NormalizeNetworkTelemetry(networktarget.NetworkTelemetryInput{
		NetworkID:      "aptos-mainnet",
		SubjectKind:    "network",
		SubjectID:      "aptos-mainnet",
		Source:         "aptos-rest-ledger-index:node.example",
		ObservedAt:     time.UnixMilli(1780000000000),
		EvidenceStatus: "verified",
	})
	if err != nil {
		t.Fatal(err)
	}
	aptos, err := BuildAptosIdentityEvent("aptos-identity-adapter", networktarget.AptosMainnetIdentityProbeResult{
		SchemaVersion:     networktarget.NetworkTelemetrySchemaVersion,
		Observation:       aptosObservation,
		ChainID:           1,
		Epoch:             42,
		LedgerVersion:     1234,
		LedgerTimestamp:   1780000000000000,
		BlockHeight:       1000,
		NodeRole:          "full_node",
		GitHash:           "abc123",
		EndpointScope:     "aptos_rest_mainnet_ledger_identity",
		AnalysisPerformed: true,
		LiveAvailability:  "checked",
	}, strings.Repeat("e", 64))
	if err != nil {
		t.Fatal(err)
	}
	if aptos.State != securityevidence.StateVerified || aptos.NetworkID != "aptos-mainnet" {
		t.Fatalf("unexpected aptos event: %#v", aptos)
	}
	if err := aptos.Verify(); err != nil {
		t.Fatal(err)
	}
}

func TestProbeAdaptersRejectIncompleteLiveObservation(t *testing.T) {
	resolution, err := networktarget.Resolve("ethereum-mainnet", "0x1111111111111111111111111111111111111111")
	if err != nil {
		t.Fatal(err)
	}
	_, err = BuildEVMAddressProbeEvent("evm-rpc-adapter", networktarget.EVMProbeResult{
		SchemaVersion:    networktarget.SchemaVersion,
		Resolution:       resolution,
		EvidenceStatus:   "observed",
		LiveAvailability: "not_checked",
	}, time.UnixMilli(1780000000000), strings.Repeat("f", 64))
	if err == nil {
		t.Fatal("incomplete probe was accepted")
	}
}
