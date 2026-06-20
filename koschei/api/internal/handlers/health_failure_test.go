package handlers

import "testing"

func TestClassifyArvisFailure(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{"", "unknown"},
		{"insert arm verdict holder_concentration: duplicate key value violates unique constraint", "verdict_insert"},
		{"insert arm event pump_sybil_radar: connection reset by peer", "event_insert"},
		{"check existing arm verdict final_verdict_engine: context deadline exceeded", "idempotency_check"},
		{"duplicate key value violates unique constraint", "duplicate_write"},
		{"insert or update violates foreign key constraint 23503", "foreign_key"},
		{"null value violates not-null constraint 23502", "schema_constraint"},
		{"relation arvis_stream_processing does not exist", "missing_schema"},
		{"rpc request timeout", "timeout"},
		{"database connection EOF", "database_or_network"},
		{"invalid json payload", "json_encoding"},
		{"unexpected processor failure", "processing_error"},
	}
	for _, tc := range cases {
		if got := classifyArvisFailure(tc.input); got != tc.want {
			t.Fatalf("classifyArvisFailure(%q)=%q want=%q", tc.input, got, tc.want)
		}
	}
}
