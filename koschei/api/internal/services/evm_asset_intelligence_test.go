package services

import (
	"strings"
	"testing"
	"time"

	"koschei/api/internal/networktarget"
)

func TestAdaptEVMAssetEvidencePreservesBoundedSurfaceClaim(t *testing.T) {
	result := networktarget.EVMAssetProbeResult{
		SchemaVersion:                 networktarget.SchemaVersion,
		Network:                       "ethereum-mainnet",
		ChainID:                       "0x1",
		ExpectedChainID:               "0x1",
		Contract:                      "0x1111111111111111111111111111111111111111",
		InterfaceState:                "erc20_like_surface_observed",
		TotalSupply:                   "1000",
		ContractBalance:               "100",
		Decimals:                      18,
		DecimalsObserved:              true,
		Symbol:                        "KOS",
		SymbolEncoding:                "abi_string",
		SymbolObserved:                true,
		TotalSupplyResponseSHA256:     strings.Repeat("a", 64),
		ContractBalanceResponseSHA256: strings.Repeat("b", 64),
		EvidenceStatus:                IntelligenceEvidenceObserved,
		LiveAvailability:              "checked",
	}
	projection, err := AdaptEVMAssetEvidence(result, time.Unix(1700000000, 0).UTC())
	if err != nil {
		t.Fatal(err)
	}
	attrs := projection.Evidence.Attributes
	if attrs["interface_state"] != "erc20_like_surface_observed" || attrs["total_supply"] != "1000" ||
		attrs["contract_balance"] != "100" || attrs["decimals"] != uint64(18) || attrs["symbol"] != "KOS" {
		t.Fatalf("asset evidence missing: %#v", attrs)
	}
	if attrs["standards_compliance_claimed"] != false || attrs["safety_claimed"] != false {
		t.Fatalf("asset surface was overclaimed: %#v", attrs)
	}
}
