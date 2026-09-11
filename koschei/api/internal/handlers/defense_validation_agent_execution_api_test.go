package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"koschei/api/internal/services"
)

func TestDefenseValidationAgentExecutionEvidenceV1ReturnsEvidenceOnlyTraces(t *testing.T) {
	request := defenseValidationAPITestRequest(t)
	registry, err := json.Marshal(map[string]string{
		request.Controls[0].CollectorRef: request.Controls[0].CollectorPublicKey,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv(defenseValidationTrustedCollectorsEnv, string(registry))

	body, err := json.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}
	recorder := httptest.NewRecorder()
	httpRequest := httptest.NewRequest(http.MethodPost, "/api/v1/agent-execution/evidence", strings.NewReader(string(body)))
	(&Handler{}).DefenseValidationAgentExecutionEvidenceV1(recorder, httpRequest)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	var response defenseValidationAgentExecutionEvidenceResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if !response.OK || response.ContractVersion != services.AgentExecutionEvidenceContractVersion {
		t.Fatalf("unexpected response: %#v", response)
	}
	if response.TraceCount != len(request.Cases) || len(response.Traces) != len(request.Cases) {
		t.Fatalf("trace count=%d traces=%d want=%d", response.TraceCount, len(response.Traces), len(request.Cases))
	}
	for _, trace := range response.Traces {
		if trace.ActorSubjectID != "" || trace.TraceStatus != services.IntelligenceEvidenceUnverified {
			t.Fatalf("validation-only response manufactured actor completeness: %#v", trace)
		}
	}
}

func TestDefenseValidationAgentExecutionEvidenceV1FailsClosedWithoutTrustedCollectors(t *testing.T) {
	t.Setenv(defenseValidationTrustedCollectorsEnv, "")
	body, err := json.Marshal(defenseValidationAPITestRequest(t))
	if err != nil {
		t.Fatal(err)
	}
	recorder := httptest.NewRecorder()
	httpRequest := httptest.NewRequest(http.MethodPost, "/api/v1/agent-execution/evidence", strings.NewReader(string(body)))
	(&Handler{}).DefenseValidationAgentExecutionEvidenceV1(recorder, httpRequest)
	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
}
