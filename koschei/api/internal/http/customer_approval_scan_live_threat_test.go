package http

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSOCApprovalThreatActualRPCCallPath(t *testing.T) {
	for _, tc := range []struct {
		name, allowance, code, finding, evidence, severity string
		badStorage, wrongChain                             bool
		wantHTTP                                           int
	}{
		{
			name:      "unlimited_eip7702_delegated_spender",
			allowance: "0x" + strings.Repeat("f", 64),
			code:      "0xef0100" + strings.Repeat("4", 40),
			finding:   "unlimited_allowance_with_mutable_spender_authority",
			evidence:  "observed_latest_unpinned", severity: "high", wantHTTP: 200,
		},
		{
			name:      "finite_non_delegated_spender",
			allowance: "0x" + strings.Repeat("0", 62) + "64",
			code:      "0x60006000",
			finding:   "no_compound_spender_exposure_observed",
			evidence:  "observed_latest_unpinned", severity: "none", wantHTTP: 200,
		},
		{
			name:       "unavailable_proxy_evidence_withholds",
			allowance:  "0x" + strings.Repeat("f", 64),
			code:       "0xef0100" + strings.Repeat("4", 40),
			badStorage: true,
			finding:    "withheld", evidence: "incomplete", severity: "unknown", wantHTTP: 200,
		},
		{
			name:       "wrong_chain_refuses_untrusted_rpc",
			allowance:  "0x" + strings.Repeat("f", 64),
			code:       "0xef0100" + strings.Repeat("4", 40),
			wrongChain: true, wantHTTP: 502,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls := map[string]int{}
			server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var req struct {
					ID     int    `json:"id"`
					Method string `json:"method"`
					Params []any  `json:"params"`
				}
				if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
					t.Errorf("decode JSON RPC: %v", err)
					w.WriteHeader(http.StatusBadRequest)
					return
				}
				calls[req.Method]++
				var value string
				switch req.Method {
				case "eth_chainId":
					value = "0x1"
					if tc.wrongChain {
						value = "0x2105"
					}
				case "eth_call":
					value = tc.allowance
				case "eth_getCode":
					value = tc.code
				case "eth_getStorageAt":
					value = "0x" + strings.Repeat("0", 64)
					if tc.badStorage {
						value = "0x01"
					}
				default:
					t.Errorf("unexpected JSON RPC method %q", req.Method)
					w.WriteHeader(http.StatusBadRequest)
					return
				}
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": req.ID, "result": value})
			}))
			defer server.Close()
			t.Setenv("ETHEREUM_RPC_URL", server.URL)
			body := `{"network":"ethereum-mainnet","token":"0x1111111111111111111111111111111111111111","owner":"0x2222222222222222222222222222222222222222","spender":"0x3333333333333333333333333333333333333333"}`
			request := httptest.NewRequest(http.MethodPost, "/api/scan/approval", strings.NewReader(body))
			request.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()
			customerApprovalScanWithRPCClient(rec, request, server.Client())
			if rec.Code != tc.wantHTTP {
				t.Fatalf("live RPC path HTTP=%d want=%d body=%s", rec.Code, tc.wantHTTP, rec.Body.String())
			}
			if calls["eth_chainId"] == 0 {
				t.Fatal("live RPC path never verified the chain identity")
			}
			if tc.wrongChain {
				if calls["eth_call"] != 0 || strings.Contains(rec.Body.String(), "threat_assessment") ||
					!strings.Contains(rec.Body.String(), `"analysis_performed":false`) {
					t.Fatalf("wrong chain promoted into an assessed scan: %s", rec.Body.String())
				}
				return
			}
			if calls["eth_call"] != 1 || calls["eth_getCode"] == 0 || calls["eth_getStorageAt"] == 0 {
				t.Fatalf("live data missing: RPC calls=%+v", calls)
			}
			var result struct {
				Result struct {
					ThreatAssessment struct {
						PlaybookID       string `json:"playbook_id"`
						FindingState     string `json:"finding_state"`
						EvidenceState    string `json:"evidence_state"`
						Severity         string `json:"severity"`
						EnforcementState string `json:"enforcement_state"`
						ReportIntegrity  string `json:"report_integrity"`
					} `json:"threat_assessment"`
				} `json:"result"`
			}
			if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
				t.Fatal(err)
			}
			got := result.Result.ThreatAssessment
			if got.PlaybookID != "SOC-PB-03" || got.FindingState != tc.finding ||
				got.EvidenceState != tc.evidence || got.Severity != tc.severity ||
				got.EnforcementState != "not_connected" || got.ReportIntegrity != "unsigned_read_only_observation" {
				t.Fatalf("live RPC exposure result is not evidence bounded: %+v", got)
			}
		})
	}
}
