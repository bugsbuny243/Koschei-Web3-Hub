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

func TestBuildEVMAddressProbeEventFromResultBindsNativeResponseDigests(t *testing.T) {
	resolution, err := networktarget.Resolve("ethereum-mainnet", "0x1111111111111111111111111111111111111111")
	if err != nil {
		t.Fatal(err)
	}
	chainDigest := strings.Repeat("a", 64)
	codeDigest := strings.Repeat("b", 64)
	result := networktarget.EVMProbeResult{
		SchemaVersion:              networktarget.SchemaVersion,
		Resolution:                 resolution,
		ChainID:                    "0x1",
		ExpectedChainID:            "0x1",
		ChainIDResponseSHA256:      chainDigest,
		ContractCodeState:          "contract_code_observed",
		ContractCodeHash:           strings.Repeat("c", 64),
		ContractCodeResponseSHA256: codeDigest,
		DelegationState:            networktarget.EVMDelegationStateNotObserved,
		AnalysisPerformed:          true,
		EvidenceStatus:             "observed",
		LiveAvailability:           "checked",
	}
	event, err := BuildEVMAddressProbeEventFromResult("evm-rpc-adapter", result, time.UnixMilli(1780000000000))
	if err != nil {
		t.Fatal(err)
	}
	if err := event.Verify(); err != nil {
		t.Fatal(err)
	}
	if len(event.SourceDigests) != 2 || event.SourceDigests[0] != chainDigest || event.SourceDigests[1] != codeDigest {
		t.Fatalf("unexpected source digests: %#v", event.SourceDigests)
	}
	facts := map[string]Fact{}
	for _, fact := range event.Facts {
		facts[fact.Key] = fact
	}
	if facts["chain_id"].EvidenceSHA256 != chainDigest || facts["expected_chain_id"].EvidenceSHA256 != chainDigest {
		t.Fatalf("chain facts lost chain response binding: %#v", facts)
	}
	if facts["contract_code_state"].EvidenceSHA256 != codeDigest ||
		facts["contract_code_sha256"].EvidenceSHA256 != codeDigest ||
		facts["delegation_state"].EvidenceSHA256 != codeDigest {
		t.Fatalf("code facts lost code response binding: %#v", facts)
	}

	result.ContractCodeResponseSHA256 = ""
	if _, err := BuildEVMAddressProbeEventFromResult("evm-rpc-adapter", result, time.UnixMilli(1780000000000)); err == nil {
		t.Fatal("missing native response digest was accepted")
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

func TestBuildBitcoinAddressProbeEventFromResultBindsNativeResponseDigests(t *testing.T) {
	resolution, err := networktarget.Resolve("bitcoin-mainnet", "1BoatSLRHtKNngkdXEeobR76b53LETtpyT")
	if err != nil {
		t.Fatal(err)
	}
	genesisDigest := strings.Repeat("a", 64)
	addressDigest := strings.Repeat("b", 64)
	result := networktarget.BitcoinProbeResult{
		SchemaVersion:         networktarget.SchemaVersion,
		Resolution:            resolution,
		GenesisHash:           strings.Repeat("0", 64),
		ExpectedGenesisHash:   strings.Repeat("0", 64),
		GenesisResponseSHA256: genesisDigest,
		ActivityState:         "activity_observed",
		ConfirmedTXCount:      12,
		MempoolTXCount:        1,
		FundedSats:            42000,
		SpentSats:             21000,
		AddressResponseSHA256: addressDigest,
		AnalysisPerformed:     true,
		EvidenceStatus:        "observed",
		LiveAvailability:      "checked",
	}
	event, err := BuildBitcoinAddressProbeEventFromResult("bitcoin-esplora-adapter", result, time.UnixMilli(1780000000000))
	if err != nil {
		t.Fatal(err)
	}
	if err := event.Verify(); err != nil {
		t.Fatal(err)
	}
	if len(event.SourceDigests) != 2 || event.SourceDigests[0] != genesisDigest || event.SourceDigests[1] != addressDigest {
		t.Fatalf("unexpected source digests: %#v", event.SourceDigests)
	}
	facts := map[string]Fact{}
	for _, fact := range event.Facts {
		facts[fact.Key] = fact
	}
	if facts["genesis_hash"].EvidenceSHA256 != genesisDigest || facts["expected_genesis_hash"].EvidenceSHA256 != genesisDigest {
		t.Fatalf("genesis facts lost native response binding: %#v", facts)
	}
	if facts["activity_state"].EvidenceSHA256 != addressDigest ||
		facts["confirmed_tx_count"].EvidenceSHA256 != addressDigest ||
		facts["funded_sats"].EvidenceSHA256 != addressDigest {
		t.Fatalf("activity facts lost native response binding: %#v", facts)
	}

	result.AddressResponseSHA256 = ""
	if _, err := BuildBitcoinAddressProbeEventFromResult("bitcoin-esplora-adapter", result, time.UnixMilli(1780000000000)); err == nil {
		t.Fatal("missing native address response digest was accepted")
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

func TestBuildMoveIdentityEventsFromResultRequireNativeResponseDigest(t *testing.T) {
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
	suiDigest := strings.Repeat("d", 64)
	suiResult := networktarget.SuiMainnetIdentityProbeResult{
		SchemaVersion:     networktarget.NetworkTelemetrySchemaVersion,
		Observation:       suiObservation,
		ChainIdentifier:   networktarget.SuiMainnetChainIdentifier,
		ResponseSHA256:    suiDigest,
		EndpointScope:     "sui_graphql_chain_identity_only",
		AnalysisPerformed: true,
		LiveAvailability:  "checked",
	}
	suiEvent, err := BuildSuiIdentityEventFromResult("sui-identity-adapter", suiResult)
	if err != nil {
		t.Fatal(err)
	}
	if len(suiEvent.SourceDigests) != 1 || suiEvent.SourceDigests[0] != suiDigest {
		t.Fatalf("unexpected sui source digests: %#v", suiEvent.SourceDigests)
	}
	suiResult.ResponseSHA256 = ""
	if _, err := BuildSuiIdentityEventFromResult("sui-identity-adapter", suiResult); err == nil {
		t.Fatal("missing Sui native response digest was accepted")
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
	aptosDigest := strings.Repeat("e", 64)
	aptosResult := networktarget.AptosMainnetIdentityProbeResult{
		SchemaVersion:     networktarget.NetworkTelemetrySchemaVersion,
		Observation:       aptosObservation,
		ChainID:           1,
		Epoch:             42,
		LedgerVersion:     1234,
		LedgerTimestamp:   1780000000000000,
		BlockHeight:       1000,
		NodeRole:          "full_node",
		GitHash:           "abc123",
		ResponseSHA256:    aptosDigest,
		EndpointScope:     "aptos_rest_mainnet_ledger_identity",
		AnalysisPerformed: true,
		LiveAvailability:  "checked",
	}
	aptosEvent, err := BuildAptosIdentityEventFromResult("aptos-identity-adapter", aptosResult)
	if err != nil {
		t.Fatal(err)
	}
	if len(aptosEvent.SourceDigests) != 1 || aptosEvent.SourceDigests[0] != aptosDigest {
		t.Fatalf("unexpected aptos source digests: %#v", aptosEvent.SourceDigests)
	}
	aptosResult.ResponseSHA256 = ""
	if _, err := BuildAptosIdentityEventFromResult("aptos-identity-adapter", aptosResult); err == nil {
		t.Fatal("missing Aptos native response digest was accepted")
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
