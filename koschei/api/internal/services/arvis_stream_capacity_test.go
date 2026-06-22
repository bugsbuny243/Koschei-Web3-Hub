package services

import "testing"

func TestArvisStreamVerdictCapacityDefaults(t *testing.T) {
	t.Setenv("ARVIS_STREAM_VERDICT_BATCH_SIZE", "")
	t.Setenv("ARVIS_STREAM_VERDICT_CONCURRENCY", "")
	if got := arvisStreamVerdictBatchSize(); got != 20 {
		t.Fatalf("batch size default = %d, want 20", got)
	}
	if got := arvisStreamVerdictConcurrency(); got != 4 {
		t.Fatalf("concurrency default = %d, want 4", got)
	}
}

func TestArvisStreamVerdictCapacityOverrides(t *testing.T) {
	t.Setenv("ARVIS_STREAM_VERDICT_BATCH_SIZE", "48")
	t.Setenv("ARVIS_STREAM_VERDICT_CONCURRENCY", "6")
	if got := arvisStreamVerdictBatchSize(); got != 48 {
		t.Fatalf("batch size override = %d, want 48", got)
	}
	if got := arvisStreamVerdictConcurrency(); got != 6 {
		t.Fatalf("concurrency override = %d, want 6", got)
	}
}

func TestArvisStreamVerdictCapacityRejectsUnsafeValues(t *testing.T) {
	t.Setenv("ARVIS_STREAM_VERDICT_BATCH_SIZE", "500")
	t.Setenv("ARVIS_STREAM_VERDICT_CONCURRENCY", "99")
	if got := arvisStreamVerdictBatchSize(); got != 20 {
		t.Fatalf("unsafe batch size = %d, want safe default 20", got)
	}
	if got := arvisStreamVerdictConcurrency(); got != 4 {
		t.Fatalf("unsafe concurrency = %d, want safe default 4", got)
	}
}
