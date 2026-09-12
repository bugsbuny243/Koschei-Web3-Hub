package networktarget

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestProbeEVMProxyAuthorityObservesStandardSlotsWithoutSafetyClaim(t *testing.T) {
	resolution, err := Resolve("ethereum-mainnet", "0x1111111111111111111111111111111111111111")
	if err != nil {
		t.Fatal(err)
	}
	implementation := "0x2222222222222222222222222222222222222222"
	admin := "0x3333333333333333333333333333333333333333"

	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req evmRPCRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatal(err)
		}
		result := "0x1"
		if req.Method == "eth_getStorageAt" {
			slot, _ := req.Params[1].(string)
			switch slot {
			case eip1967ImplementationSlot:
				result = "0x" + strings.Repeat("0", 24) + strings.TrimPrefix(implementation, "0x")
			case eip1967AdminSlot:
				result = "0x" + strings.Repeat("0", 24) + strings.TrimPrefix(admin, "0x")
			case eip1967BeaconSlot:
				result = "0x" + strings.Repeat("0", 64)
			default:
				t.Fatalf("unexpected slot %q", slot)
			}
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": req.ID, "result": result})
	}))
	defer server.Close()

	result, err := ProbeEVMProxyAuthority(context.Background(), server.Client(), server.URL, resolution)
	if err != nil {
		t.Fatal(err)
	}
	if result.State != EVMProxyAuthorityObserved {
		t.Fatalf("state=%q", result.State)
	}
	if result.Implementation != implementation || result.Admin != admin || result.Beacon != "" {
		t.Fatalf("unexpected authority result: %+v", result)
	}
	if result.ConflictingSlots {
		t.Fatalf("unexpected conflicting slots: %+v", result)
	}
	if !result.AnalysisPerformed || result.EvidenceStatus != "observed" || result.LiveAvailability != "checked" {
		t.Fatalf("observation contract broken: %+v", result)
	}
}

func TestProbeEVMProxyAuthorityDoesNotTreatEmptySlotsAsImmutabilityProof(t *testing.T) {
	resolution, err := Resolve("base-mainnet", "0x1111111111111111111111111111111111111111")
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req evmRPCRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatal(err)
		}
		result := "0x2105"
		if req.Method == "eth_getStorageAt" {
			result = "0x" + strings.Repeat("0", 64)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": req.ID, "result": result})
	}))
	defer server.Close()

	result, err := ProbeEVMProxyAuthority(context.Background(), server.Client(), server.URL, resolution)
	if err != nil {
		t.Fatal(err)
	}
	if result.State != EVMProxyAuthorityNotObserved {
		t.Fatalf("state=%q", result.State)
	}
	if result.Implementation != "" || result.Admin != "" || result.Beacon != "" {
		t.Fatalf("empty ERC-1967 slots manufactured authority: %+v", result)
	}
}

func TestProbeEVMProxyAuthorityFlagsImplementationBeaconConflict(t *testing.T) {
	resolution, err := Resolve("optimism-mainnet", "0x1111111111111111111111111111111111111111")
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req evmRPCRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatal(err)
		}
		result := "0xa"
		if req.Method == "eth_getStorageAt" {
			result = "0x" + strings.Repeat("0", 24) + strings.Repeat("4", 40)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": req.ID, "result": result})
	}))
	defer server.Close()

	result, err := ProbeEVMProxyAuthority(context.Background(), server.Client(), server.URL, resolution)
	if err != nil {
		t.Fatal(err)
	}
	if !result.ConflictingSlots {
		t.Fatalf("expected implementation/beacon conflict: %+v", result)
	}
}
