package handlers

import "testing"

func TestUnifiedMintOwnerEvidenceProjectionFailClosed(t *testing.T) {
	tests := []struct {
		name     string
		source   map[string]any
		verified bool
	}{
		{name: "nil"},
		{name: "missing", source: map[string]any{}},
		{name: "unverified", source: map[string]any{"mint_owner_program_evidence_status": "unverified", "mint_owner_program": token2022ProgramID}},
		{name: "verified", source: map[string]any{"mint_owner_program_evidence_status": "verified", "mint_owner_program": token2022ProgramID}, verified: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := unifiedMintOwnerEvidenceProjection(tc.source)
			_, classified := got["token_2022"]
			if classified != tc.verified {
				t.Fatalf("classification=%v verified=%v", got, tc.verified)
			}
			if tc.verified && got["token_2022"] != true {
				t.Fatalf("verified Token-2022 owner not preserved: %v", got)
			}
		})
	}
}
