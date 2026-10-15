package handlers

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestUnifiedToken2022ExtensionEvidenceFromMintRPC(t *testing.T) {
	const pyusd = "2b1kV6DkPAnxd5ixfnxCpjxmKwqjjaYmCZfHsFu24GXo"
	const token2022 = "TokenzQdBNbLqP5VEhdkAS6EPFLC1PHnBqCXEpPxuEb"
	const classic = "TokenkegQfeZyiNwAJbNbGKPFXCWuBvf9Ss623VQ5DA"
	tests := []struct {
		name, owner, parsedInfo, expected string
		space                             int
		complete                          bool
		count                             int
	}{
		{
			name:       "parsed_token2022_extensions",
			owner:      token2022,
			parsedInfo: `{"extensions":[{"extension":"transferHook","state":{"programId":"HookProgram"}},{"extension":"permanentDelegate","state":{"delegate":"DelegateWallet"}}]}`,
			space:      866,
			expected:   "resolved",
			complete:   true,
			count:      2,
		},
		{
			name:       "unresolved_extension_space",
			owner:      token2022,
			parsedInfo: `{}`,
			space:      866,
			expected:   "token_2022_extensions_unresolved",
		},
		{
			name:       "unknown_account_space",
			owner:      token2022,
			parsedInfo: `{}`,
			expected:   "token_2022_extensions_unresolved",
		},
		{
			name:       "partially_parsed_extensions",
			owner:      token2022,
			parsedInfo: `{"extensions":[{"extension":"transferHook"},{"state":{"unknown":"entry"}}]}`,
			space:      866,
			expected:   "token_2022_extensions_unresolved",
		},
		{
			name:       "extensions_wrong_json_shape",
			owner:      token2022,
			parsedInfo: `{"extensions":"not_an_array"}`,
			space:      866,
			expected:   "token_2022_extensions_unresolved",
		},
		{
			name:       "missing_space_even_with_parsed_extensions",
			owner:      token2022,
			parsedInfo: `{"extensions":[{"extension":"transferHook"}]}`,
			expected:   "token_2022_extensions_unresolved",
		},
		{
			name:       "impossible_extensions_in_classic_sized_mint",
			owner:      token2022,
			parsedInfo: `{"extensions":[{"extension":"transferHook"}]}`,
			space:      82,
			expected:   "token_2022_extensions_unresolved",
		},
		{
			name:       "verified_empty_token2022_mint",
			owner:      token2022,
			parsedInfo: `{"extensions":[]}`,
			space:      82,
			expected:   "resolved_empty",
			complete:   true,
		},
		{
			name:       "classic_mint",
			owner:      classic,
			parsedInfo: `{}`,
			space:      82,
			expected:   "not_applicable",
		},
		{
			name:       "unknown_owner",
			owner:      "OtherProgram111111111111111111111111111111",
			parsedInfo: `{}`,
			expected:   "unverified",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_, _ = fmt.Fprintf(w, `{"jsonrpc":"2.0","id":1,"result":{"value":{"owner":%q,"data":{"parsed":{"type":"mint","info":%s},"space":%d}}}}`, tc.owner, tc.parsedInfo, tc.space)
			}))
			defer server.Close()
			t.Setenv("SOLANA_RPC_URL", server.URL)
			source := (&Handler{}).collectUnifiedMintOwnerEvidence(context.Background(), pyusd, "solana-mainnet")
			got := unifiedToken2022ExtensionProjection(source)
			if got["status"] != tc.expected || got["evidence_complete"] != tc.complete || got["extension_count"] != tc.count {
				t.Fatalf("projection=%v wanted status=%s complete=%v count=%d", got, tc.expected, tc.complete, tc.count)
			}
			if !tc.complete && got["extension_count"] != 0 {
				t.Fatalf("unresolved observations cannot invent extension counts: %v", got)
			}
			if tc.complete {
				names, ok := got["extension_names"].([]string)
				if !ok || len(names) != tc.count {
					t.Fatalf("missing parsed extension names: %v", got)
				}
			}
			if tc.expected == "token_2022_extensions_unresolved" && got["limitation_code"] != "TOKEN_2022_EXTENSIONS_UNRESOLVED" {
				t.Fatalf("missing named parser limitation: %v", got)
			}
		})
	}
}

func TestUnifiedToken2022ProjectionCannotTrustUnverifiedOwner(t *testing.T) {
	untrusted := map[string]any{
		"mint_owner_program_evidence_status": "unverified",
		"token_2022":                         true,
		"token_2022_extension_evidence": map[string]any{
			"status": "resolved", "evidence_complete": true,
			"extension_count": 1, "extension_names": []string{"permanentDelegate"},
		},
	}
	got := unifiedToken2022ExtensionProjection(untrusted)
	if got["status"] != "unverified" || got["evidence_complete"] != false {
		t.Fatalf("unverified owner incorrectly upgraded to verified extension: %v", got)
	}
}
