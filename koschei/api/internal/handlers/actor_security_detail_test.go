package handlers

import (
	"context"
	"testing"

	"koschei/api/internal/services"
)

func TestActorSecurityIntelligenceForDetailWithNoEvidenceStaysUnavailable(t *testing.T) {
	h := &Handler{}
	actor := h.actorSecurityIntelligenceForDetail(
		context.Background(),
		"11111111111111111111111111111111",
		"solana-mainnet",
		map[string]any{"available": false},
		services.HolderRoleAnalysis{},
		services.HolderClusterAnalysis{},
		services.TokenMarketSnapshot{},
	)
	if got := actor["status"]; got != "actor_evidence_unavailable" {
		t.Fatalf("status = %v, want actor_evidence_unavailable", got)
	}
	if got := actor["available"]; got != false {
		t.Fatalf("available = %v, want false", got)
	}
	if got := actor["source"]; got != "full_radar_fresh_holder_evidence" {
		t.Fatalf("source = %v", got)
	}
}
