package handlers

import "testing"

func TestMintOwnerProgramEvidenceTriState(t *testing.T) {
	tests := []struct {
		name, owner, status             string
		hasClassification, token2022 bool
	}{
		{"classic", "TokenkegQfeZyiNwAJbNbGKPFXCWuBvf9Ss623VQ5DA", "verified", true, false},
		{"token2022", token2022ProgramID, "verified", true, true},
		{"missing", "", "unverified", false, false},
		{"unknown", "UnknownProgram111111111111111111111111111", "unverified", false, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := mintOwnerProgramEvidence(tc.owner)
			if got["mint_owner_program_evidence_status"] != tc.status {
				t.Fatalf("status=%v want=%s", got, tc.status)
			}
			value, exists := got["token_2022"]
			if exists != tc.hasClassification || (exists && value != tc.token2022) {
				t.Fatalf("classification=%v exists=%v", value, exists)
			}
			_, hasOwner := got["mint_owner_program"]
			if hasOwner != tc.hasClassification {
				t.Fatalf("unverified owner must not be presented as a verified mint owner: %v", got)
			}
		})
	}
}
