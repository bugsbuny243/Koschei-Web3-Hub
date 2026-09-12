package services

import "testing"

func TestClassifyCustomerScanTargetRequiresNetworkForEVMAddress(t *testing.T) {
	target, err := ClassifyCustomerScanTarget("0x1111111111111111111111111111111111111111", "")
	if err != nil {
		t.Fatal(err)
	}
	if target.Kind != CustomerScanTargetEVMAddress || target.Route != CustomerScanRouteEVMProbe || !target.RequiresNetwork {
		t.Fatalf("unexpected target: %#v", target)
	}
}

func TestClassifyCustomerScanTargetDoesNotInferTxNetwork(t *testing.T) {
	target, err := ClassifyCustomerScanTarget("0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", "")
	if err != nil {
		t.Fatal(err)
	}
	if target.Kind != CustomerScanTargetTxHash || target.Route != CustomerScanRouteTxLookup || !target.RequiresNetwork {
		t.Fatalf("unexpected target: %#v", target)
	}
}

func TestClassifyCustomerScanTargetSolanaSyntaxOnly(t *testing.T) {
	target, err := ClassifyCustomerScanTarget("11111111111111111111111111111111", "solana-mainnet")
	if err != nil {
		t.Fatal(err)
	}
	if target.Kind != CustomerScanTargetSolana || target.Route != CustomerScanRouteSolanaIntel || target.Classification != "syntax_only" {
		t.Fatalf("unexpected target: %#v", target)
	}
}

func TestClassifyCustomerScanTargetUnknownFailsClosed(t *testing.T) {
	target, err := ClassifyCustomerScanTarget("not-a-web3-target", "")
	if err != nil {
		t.Fatal(err)
	}
	if target.Kind != CustomerScanTargetUnknown || target.Route != CustomerScanRouteUnresolved || len(target.Reasons) != 1 || target.Reasons[0] != "TARGET_TYPE_UNRESOLVED" {
		t.Fatalf("unexpected target: %#v", target)
	}
}
