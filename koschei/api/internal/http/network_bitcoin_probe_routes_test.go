package http

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestBitcoinProbeRequiresConfiguredEsploraEndpoint(t *testing.T) {
	t.Setenv("BITCOIN_ESPLORA_URL", "")
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/fabric/networks/probe", strings.NewReader(`{"network":"bitcoin-mainnet","address":"1BoatSLRHtKNngkdXEeobR76b53LETtpyT"}`))
	request.Header.Set("Content-Type", "application/json")

	networkTargetProbe(recorder, request)
	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	body := recorder.Body.String()
	if !strings.Contains(body, `"error":"bitcoin_esplora_configuration_required"`) || !strings.Contains(body, `"live_availability":"configuration_required"`) {
		t.Fatalf("unexpected fail-closed response: %s", body)
	}
}
