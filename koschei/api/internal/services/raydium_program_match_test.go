package services

import "testing"

func TestIsKnownRaydiumProgram(t *testing.T) {
	cases := []struct {
		name    string
		program string
		want    bool
	}{
		{name: "canonical", program: defaultRaydiumProgramID, want: true},
		{name: "legacy program", program: legacyRaydiumProgramID, want: true},
		{name: "legacy source", program: legacyRaydiumSourceID, want: true},
		{name: "named program", program: "raydium_amm", want: true},
		{name: "unrelated", program: defaultPumpProgramID, want: false},
		{name: "empty", program: "", want: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := isKnownRaydiumProgram(tc.program); got != tc.want {
				t.Fatalf("isKnownRaydiumProgram(%q) = %t, want %t", tc.program, got, tc.want)
			}
		})
	}
}

func TestParseArvisTransactionMapRecognizesCanonicalRaydiumProgram(t *testing.T) {
	out := arvisTransactionEvidence{TokenBalanceChanges: map[string]float64{}, LamportDeltas: map[string]int64{}}
	parseArvisTransactionMap(map[string]any{
		"transaction": map[string]any{
			"message": map[string]any{
				"accountKeys": []any{defaultRaydiumProgramID},
				"instructions": []any{
					map[string]any{"programId": defaultRaydiumProgramID},
				},
			},
		},
		"meta": map[string]any{
			"preBalances":  []any{float64(1)},
			"postBalances": []any{float64(1)},
			"logMessages":  []any{},
		},
	}, &out)
	if !out.RaydiumRelated {
		t.Fatal("canonical Raydium program was not recognized")
	}
}
