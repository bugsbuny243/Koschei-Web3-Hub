package http

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"koschei/api/internal/handlers"
	"koschei/api/internal/runtimehealth"
)

func TestChannelWebhooksRejectMissingAndIncorrectSecrets(t *testing.T) {
	for _, test := range []struct {
		name, env, path string
		handler         http.HandlerFunc
	}{
		{"telegram", "TELEGRAM_WEBHOOK_SECRET", "/webhooks/telegram", tradePITelegramWebhook},
		{"whatsapp", "WHATSAPP_APP_SECRET", "/webhooks/whatsapp", tradePIWhatsAppReceive},
	} {
		t.Run(test.name, func(t *testing.T) {
			for _, secret := range []string{"", "test-webhook-secret"} {
				t.Setenv(test.env, secret)
				res := httptest.NewRecorder()
				test.handler(res, httptest.NewRequest(http.MethodPost, test.path, strings.NewReader(`{}`)))
				if res.Code != http.StatusUnauthorized {
					t.Fatalf("missing/invalid credential accepted: status=%d", res.Code)
				}
			}
		})
	}
}

func TestFabricProbeBudgetStopsBeforeProvider(t *testing.T) {
	calls := 0
	handler := fabricProbeProtection(fabricConfig{}, func(w http.ResponseWriter, r *http.Request) { calls++; w.WriteHeader(http.StatusNoContent) })
	for i := 0; i < 11; i++ {
		request := httptest.NewRequest(http.MethodPost, "/fabric/networks/probe", nil)
		request.RemoteAddr = "198.51.100.81:1234"
		response := httptest.NewRecorder()
		handler(response, request)
		want := http.StatusNoContent
		if i == 10 {
			want = http.StatusTooManyRequests
		}
		if response.Code != want {
			t.Fatalf("request %d status=%d want=%d", i, response.Code, want)
		}
	}
	if calls != 10 {
		t.Fatalf("exhausted budget reached provider: calls=%d", calls)
	}
}

func TestFabricProbeConcurrencyIsBounded(t *testing.T) {
	entered, release := make(chan struct{}, 4), make(chan struct{})
	var once sync.Once
	unblock := func() { once.Do(func() { close(release) }) }
	defer unblock()
	handler := fabricProbeProtection(fabricConfig{}, func(w http.ResponseWriter, r *http.Request) {
		entered <- struct{}{}
		<-release
		w.WriteHeader(http.StatusNoContent)
	})
	var wg sync.WaitGroup
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			request := httptest.NewRequest(http.MethodPost, "/fabric/networks/probe", nil)
			request.RemoteAddr = fmt.Sprintf("198.51.100.%d:1234", 90+i)
			handler(httptest.NewRecorder(), request)
		}(i)
	}
	for i := 0; i < 4; i++ {
		<-entered
	}
	request := httptest.NewRequest(http.MethodPost, "/fabric/networks/probe", nil)
	request.RemoteAddr = "198.51.100.95:1234"
	response := httptest.NewRecorder()
	handler(response, request)
	if response.Code != http.StatusTooManyRequests || !strings.Contains(response.Body.String(), "probe_concurrency_exceeded") {
		t.Fatalf("unbounded probe: %d %s", response.Code, response.Body.String())
	}
	unblock()
	wg.Wait()
}

func TestCampaignOwnerSurfaceFailsClosedBeforeStorage(t *testing.T) {
	t.Setenv("OWNER_SECRET", "test-campaign-owner")
	t.Setenv("KOSCHEI_OWNER_SECRET", "")
	h := &handlers.Handler{}
	mux := http.NewServeMux()
	registerGlobalCampaignOwnerRoutes(mux, h, t.TempDir())
	for _, path := range []string{"/api/owner/campaigns", "/owner/campaigns"} {
		res := httptest.NewRecorder()
		mux.ServeHTTP(res, httptest.NewRequest(http.MethodGet, path, nil))
		if res.Code != http.StatusNotFound {
			t.Fatalf("anonymous campaign surface %s: %d", path, res.Code)
		}
	}
	req := httptest.NewRequest(http.MethodGet, "/api/owner/campaigns", nil)
	req.Header.Set("X-Owner-Secret", "test-campaign-owner")
	res := httptest.NewRecorder()
	mux.ServeHTTP(res, req)
	if res.Code != http.StatusServiceUnavailable || !strings.Contains(res.Body.String(), "campaign_store_unavailable") {
		t.Fatalf("missing storage reported healthy: %d %s", res.Code, res.Body.String())
	}
}

func TestCampaignAndDemoStatelessBootRouting(t *testing.T) {
	t.Setenv("APP_ENV", "development")
	t.Setenv("OWNER_SECRET", "test-campaign-owner")
	t.Setenv("KOSCHEI_OWNER_SECRET", "")
	server := NewServer(nil, "", "", "", t.TempDir())
	res := httptest.NewRecorder()
	server.ServeHTTP(res, httptest.NewRequest(http.MethodGet, "/api/owner/campaigns", nil))
	if res.Code != http.StatusNotFound {
		t.Fatalf("stateless API bypassed owner auth: %d", res.Code)
	}
	req := httptest.NewRequest(http.MethodGet, "/api/owner/campaigns", nil)
	req.Header.Set("X-Owner-Secret", "test-campaign-owner")
	res = httptest.NewRecorder()
	server.ServeHTTP(res, req)
	if res.Code != http.StatusServiceUnavailable || !strings.Contains(res.Body.String(), "campaign_store_unavailable") {
		t.Fatalf("owner missing-storage contract bypassed: %d %s", res.Code, res.Body.String())
	}
	res = httptest.NewRecorder()
	server.ServeHTTP(res, httptest.NewRequest(http.MethodPost, "/api/agents/demo", strings.NewReader(`{"text":"BMW 320i","user_id":"boot-fixture"}`)))
	if res.Code != http.StatusOK || !strings.Contains(res.Body.String(), `"vehicles"`) {
		t.Fatalf("memory-only demo requires customer persistence: %d %s", res.Code, res.Body.String())
	}
}

func TestFabricIntelligenceRequiresOwnerBeforeRPCOrPersistence(t *testing.T) {
	t.Setenv("OWNER_SECRET", "test-owner-secret")
	t.Setenv("KOSCHEI_OWNER_SECRET", "")
	t.Setenv("ETHEREUM_RPC_URL", "https://unused.invalid/private-provider-token")
	res := httptest.NewRecorder()
	MountFabric(http.NotFoundHandler()).ServeHTTP(res, httptest.NewRequest(http.MethodPost, "/fabric/networks/probe/intelligence", strings.NewReader(`{}`)))
	if res.Code != http.StatusNotFound {
		t.Fatalf("anonymous writer reached probe: %d %s", res.Code, res.Body.String())
	}
}

func TestPublicRuntimeHealthRedactsProviderErrors(t *testing.T) {
	registry := runtimehealth.New()
	registry.Register("rpc", "network_telemetry", "ethereum-mainnet", true)
	registry.Failure("rpc", errors.New("Post https://rpc.invalid/private-provider-token?key=private-query-key: timeout"))
	res := httptest.NewRecorder()
	securityCenterRuntimeHealth(registry)(res, httptest.NewRequest(http.MethodGet, "/fabric/security-center/runtime-health", nil))
	for _, secret := range []string{"private-provider-token", "private-query-key", "https://rpc.invalid"} {
		if strings.Contains(res.Body.String(), secret) {
			t.Fatalf("public health leaked provider detail %q", secret)
		}
	}
	if registry.Snapshot().Entries[0].LastError == "component_check_failed" {
		t.Fatal("public projection mutated internal evidence")
	}
}
