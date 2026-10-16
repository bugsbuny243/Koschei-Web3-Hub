package networktarget

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestProbeEVMAllowanceObservesCurrentAllowanceWithoutPromotingHistory(t *testing.T) {
	owner := "0x2222222222222222222222222222222222222222"
	spender := "0x3333333333333333333333333333333333333333"
	token := "0x1111111111111111111111111111111111111111"
	wantData := "0x" + erc20AllowanceSelector + abiAddressWord(owner) + abiAddressWord(spender)

	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req evmRPCRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatal(err)
		}
		w.Header().Set("Content-Type", "application/json")
		switch req.Method {
		case "eth_chainId":
			_ = json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": req.ID, "result": "0x1"})
		case "eth_call":
			if len(req.Params) != 2 || req.Params[1] != "latest" {
				t.Fatalf("unexpected eth_call params: %#v", req.Params)
			}
			call, ok := req.Params[0].(map[string]any)
			if !ok {
				t.Fatalf("unexpected call object: %#v", req.Params[0])
			}
			if call["to"] != token || call["data"] != wantData {
				t.Fatalf("unexpected eth_call object: %#v", call)
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": req.ID, "result": "0x" + strings.Repeat("0", 62) + "64"})
		default:
			t.Fatalf("unexpected method %q", req.Method)
		}
	}))
	defer server.Close()

	result, err := ProbeEVMAllowance(t.Context(), server.Client(), server.URL, "ethereum-mainnet", token, owner, spender)
	if err != nil {
		t.Fatal(err)
	}
	if result.Amount != "100" || result.Unlimited {
		t.Fatalf("unexpected allowance result: %+v", result)
	}
	if result.EvidenceStatus != "observed" || result.LiveAvailability != "checked" {
		t.Fatalf("unexpected evidence state: %+v", result)
	}
}

func TestProbeEVMAllowanceRejectsWrongChain(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req evmRPCRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatal(err)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": req.ID, "result": "0xa"})
	}))
	defer server.Close()

	_, err := ProbeEVMAllowance(
		t.Context(),
		server.Client(),
		server.URL,
		"ethereum-mainnet",
		"0x1111111111111111111111111111111111111111",
		"0x2222222222222222222222222222222222222222",
		"0x3333333333333333333333333333333333333333",
	)
	if err == nil || !strings.Contains(err.Error(), "evm_rpc_network_mismatch") {
		t.Fatalf("expected network mismatch, got %v", err)
	}
}

func TestParseEVMUint256RejectsOverflow(t *testing.T) {
	if _, err := parseEVMUint256("0x1" + strings.Repeat("0", 64)); err == nil {
		t.Fatal("expected uint256 overflow to fail closed")
	}
}

func TestParseEVMAllowanceABIWordRequiresExactUint256Word(t *testing.T) {
	for _, tc := range []struct {
		name  string
		value string
		valid bool
	}{
		{"zero_full_word", "0x" + strings.Repeat("0", 64), true},
		{"finite_full_word", "0x" + strings.Repeat("0", 62) + "64", true},
		{"maximum_uint256", "0x" + strings.Repeat("f", 64), true},
		{"truncated_zero", "0x0", false},
		{"truncated_integer", "0x64", false},
		{"empty_word", "0x", false},
		{"odd_length_word", "0x" + strings.Repeat("0", 63), false},
		{"oversized_word", "0x" + strings.Repeat("0", 65), false},
		{"non_hex_word", "0x" + strings.Repeat("0", 63) + "g", false},
		{"not_a_hex_word", "0", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			amount, err := parseEVMAllowanceABIWord(tc.value)
			if (err == nil) != tc.valid {
				t.Fatalf("value %q: expected valid=%t, amount=%v err=%v", tc.value, tc.valid, amount, err)
			}
			if tc.valid && (amount == nil || amount.Sign() < 0 || amount.BitLen() > 256) {
				t.Fatalf("valid ABI observation failed numeric validation: %v", amount)
			}
		})
	}
}

func TestProbeEVMAllowanceRejectsTruncatedZeroAsObservedPermission(t *testing.T) {
	owner := "0x2222222222222222222222222222222222222222"
	spender := "0x3333333333333333333333333333333333333333"
	token := "0x1111111111111111111111111111111111111111"
	for _, result := range []string{"0x0", "0x", "0x" + strings.Repeat("0", 63)} {
		t.Run(result, func(t *testing.T) {
			server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var req evmRPCRequest
				if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
					t.Errorf("decode RPC request: %v", err)
					return
				}
				w.Header().Set("Content-Type", "application/json")
				value := result
				if req.Method == "eth_chainId" {
					value = "0x1"
				} else if req.Method != "eth_call" {
					t.Errorf("unexpected RPC method %s", req.Method)
					return
				}
				_ = json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": req.ID, "result": value})
			}))
			defer server.Close()
			observation, err := ProbeEVMAllowance(t.Context(), server.Client(), server.URL, "ethereum-mainnet", token, owner, spender)
			if err == nil || !strings.Contains(err.Error(), "evm_allowance_result_invalid") {
				t.Fatalf("malformed ABI must be unavailable, not a clean allowance: observation=%+v err=%v", observation, err)
			}
			if observation.EvidenceStatus == "observed" || observation.LiveAvailability == "checked" {
				t.Fatalf("malformed response promoted to observed: %+v", observation)
			}
		})
	}
}
