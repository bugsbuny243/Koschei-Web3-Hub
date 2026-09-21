package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestOwnerUnifiedRadarRouteCoversVerifiedTargetKinds(t *testing.T) {
	cases := []struct {
		targetType string
		want       string
	}{
		{radarTargetTokenMint, "token"},
		{radarTargetWallet, "wallet"},
		{radarTargetTokenAccount, "wallet"},
		{radarTargetProgram, "program"},
		{radarTargetTransactionSignature, "transaction"},
		{radarTargetProgramData, "program_artifact"},
		{radarTargetProgramBuffer, "program_artifact"},
		{radarTargetProgramLoaderAccount, "program_artifact"},
		{radarTargetUnknown, "classification_gap"},
	}
	for _, tc := range cases {
		if got := ownerUnifiedRadarRoute(radarTargetClassification{Type: tc.targetType}); got != tc.want {
			t.Fatalf("type=%s route=%s want=%s", tc.targetType, got, tc.want)
		}
	}
}

func TestOwnerUnifiedProgramArtifactRadarReturnsEvidenceGap(t *testing.T) {
	recorder := httptest.NewRecorder()
	classification := radarTargetClassification{
		Type:     radarTargetProgramData,
		Status:   "verified_rpc_observation",
		Evidence: "program data account",
	}
	(&Handler{}).ownerUnifiedProgramArtifactRadar(recorder, "ProgramDataTarget", "solana-mainnet", classification)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status=%d want=%d body=%s", recorder.Code, http.StatusOK, recorder.Body.String())
	}
	var payload map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if payload["ok"] != true || payload["status"] != "evidence_gap" || payload["investigation_kind"] != "program_deployment_artifact" {
		t.Fatalf("unexpected payload: %#v", payload)
	}
	finalVerdict, _ := payload["final_verdict"].(map[string]any)
	if finalVerdict["withheld"] != true || finalVerdict["signed"] != false {
		t.Fatalf("program artifact verdict must be withheld and unsigned: %#v", finalVerdict)
	}
}
