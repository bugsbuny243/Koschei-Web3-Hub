package networktarget

import (
	"errors"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestProbeEVMERC20AssetObservesMethodSurfaceWithoutComplianceClaim(t *testing.T) {
	resolution, err := Resolve("ethereum-mainnet", "0x1111111111111111111111111111111111111111")
	if err != nil {
		t.Fatal(err)
	}
	base := EVMProbeResult{
		SchemaVersion:     SchemaVersion,
		Resolution:        resolution,
		ChainID:           "0x1",
		ExpectedChainID:   "0x1",
		ContractCodeState: "contract_code_observed",
		AnalysisPerformed: true,
		EvidenceStatus:    "observed",
		LiveAvailability:  "checked",
	}
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req evmRPCRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatal(err)
		}
		call, ok := req.Params[0].(map[string]any)
		if !ok {
			t.Fatalf("unexpected call object: %#v", req.Params)
		}
		data, _ := call["data"].(string)
		var result string
		switch {
		case data == "0x"+evmERC20TotalSupplySelector:
			result = "0x3e8"
		case strings.HasPrefix(data, "0x"+evmERC20BalanceOfSelector):
			result = "0x64"
		case data == "0x"+evmERC20DecimalsSelector:
			result = "0x12"
		case data == "0x"+evmERC20SymbolSelector:
			result = abiStringTestValue("KOS")
		case data == "0x"+evmERC20NameSelector:
			result = abiStringTestValue("Koschei Test")
		default:
			t.Fatalf("unexpected eth_call data %q", data)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": req.ID, "result": result})
	}))
	defer server.Close()

	result, err := ProbeEVMERC20Asset(t.Context(), server.Client(), server.URL, base)
	if err != nil {
		t.Fatal(err)
	}
	if result.InterfaceState != "erc20_like_surface_observed" || result.TotalSupply != "1000" || result.ContractBalance != "100" {
		t.Fatalf("unexpected asset result: %+v", result)
	}
	if !result.DecimalsObserved || result.Decimals != 18 || !result.SymbolObserved || result.Symbol != "KOS" ||
		!result.NameObserved || result.Name != "Koschei Test" {
		t.Fatalf("metadata evidence missing: %+v", result)
	}
	if len(result.TotalSupplyResponseSHA256) != 64 || len(result.ContractBalanceResponseSHA256) != 64 {
		t.Fatalf("core response digests missing: %+v", result)
	}
}

func TestProbeEVMERC20AssetRequiresTwoCoreMethods(t *testing.T) {
	resolution, err := Resolve("ethereum-mainnet", "0x1111111111111111111111111111111111111111")
	if err != nil {
		t.Fatal(err)
	}
	base := EVMProbeResult{
		Resolution: resolution, ChainID: "0x1", ExpectedChainID: "0x1",
		ContractCodeState: "contract_code_observed", AnalysisPerformed: true,
		EvidenceStatus: "observed", LiveAvailability: "checked",
	}
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req evmRPCRequest
		_ = json.NewDecoder(r.Body).Decode(&req)
		call := req.Params[0].(map[string]any)
		data := call["data"].(string)
		if data == "0x"+evmERC20TotalSupplySelector {
			_ = json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": req.ID, "result": "0x1"})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"jsonrpc": "2.0", "id": req.ID,
			"error": map[string]any{"code": -32000, "message": "execution reverted"},
		})
	}))
	defer server.Close()
	_, err = ProbeEVMERC20Asset(t.Context(), server.Client(), server.URL, base)
	if !errors.Is(err, ErrEVMERC20SurfaceNotObserved) {
		t.Fatalf("expected absent ERC20-like surface, got %v", err)
	}
}

func abiStringTestValue(value string) string {
	data := []byte(value)
	padded := ((len(data) + 31) / 32) * 32
	raw := make([]byte, 64+padded)
	raw[31] = 32
	raw[63] = byte(len(data))
	copy(raw[64:], data)
	return "0x" + hex.EncodeToString(raw)
}
