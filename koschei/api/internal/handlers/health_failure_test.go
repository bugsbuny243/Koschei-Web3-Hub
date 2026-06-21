package handlers

import "testing"

func TestClassifyArvisFailure(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{"", "unknown"},
		{"insert arm verdict holder_concentration: duplicate key value violates unique constraint", "duplicate_write"},
		{"insert arm event pump_sybil_radar: connection reset by peer", "database_or_network"},
		{"check existing arm verdict final_verdict_engine: context deadline exceeded", "timeout"},
		{"duplicate key value violates unique constraint", "duplicate_write"},
		{"insert or update violates foreign key constraint 23503", "foreign_key"},
		{"null value violates not-null constraint 23502", "schema_constraint"},
		{"relation arvis_stream_processing does not exist", "missing_schema"},
		{"rpc request timeout", "timeout"},
		{"database connection EOF", "database_or_network"},
		{"invalid json payload", "json_encoding"},
		{"insert arm event pump_sybil_radar: unknown write failure", "event_insert"},
		{"insert arm verdict final_verdict_engine: unknown write failure", "verdict_insert"},
		{"check existing arm verdict final_verdict_engine: unknown read failure", "idempotency_check"},
		{"unexpected processor failure", "processing_error"},
	}
	for _, tc := range cases {
		if got := classifyArvisFailure(tc.input); got != tc.want {
			t.Fatalf("classifyArvisFailure(%q)=%q want=%q", tc.input, got, tc.want)
		}
	}
}
