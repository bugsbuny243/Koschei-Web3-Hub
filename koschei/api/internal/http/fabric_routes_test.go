package http

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestFabricSnapshotPreservesExistingProducts(t *testing.T) {
	snapshot := currentFabricSnapshot()
	if !snapshot.PreserveExisting {
		t.Fatal("Fabric must preserve existing product behavior")
	}
	if snapshot.BreakingChangesAllowed {
		t.Fatal("Fabric must not allow breaking changes by default")
	}
	if !snapshot.BackendFrontendParity {
		t.Fatal("Fabric must require backend/frontend parity")
	}
	if len(snapshot.Components) != 3 {
		t.Fatalf("components = %d, want 3", len(snapshot.Components))
	}

	want := map[string]bool{
		"koschei-web3":     false,
		"koschei-lang":     false,
		"koschei-sentinel": false,
	}
	for _, component := range snapshot.Components {
		if _, ok := want[component.Name]; ok {
			want[component.Name] = true
		}
	}
	for component, found := range want {
		if !found {
			t.Fatalf("missing Fabric component %q", component)
		}
	}
}

func TestFabricCapabilityAPI(t *testing.T) {
	mux := http.NewServeMux()
	registerFabricRoutes(mux)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/v2/fabric/capabilities", nil)
	mux.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}
	if got := recorder.Header().Get("Cache-Control"); got != "no-store" {
		t.Fatalf("Cache-Control = %q, want no-store", got)
	}

	var snapshot fabricSnapshot
	if err := json.Unmarshal(recorder.Body.Bytes(), &snapshot); err != nil {
		t.Fatalf("decode Fabric response: %v", err)
	}
	if snapshot.Workspace != "koschei-unified" {
		t.Fatalf("workspace = %q, want koschei-unified", snapshot.Workspace)
	}
}

func TestFabricOperatorSurface(t *testing.T) {
	mux := http.NewServeMux()
	registerFabricRoutes(mux)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/fabric", nil)
	mux.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}
	body := recorder.Body.String()
	for _, needle := range []string{"Koschei Fabric", "koschei-web3", "koschei-lang", "koschei-sentinel"} {
		if !strings.Contains(body, needle) {
			t.Fatalf("operator surface missing %q", needle)
		}
	}
}

func TestFabricRoutesRejectMutationMethods(t *testing.T) {
	mux := http.NewServeMux()
	registerFabricRoutes(mux)

	for _, path := range []string{"/api/v2/fabric/capabilities", "/fabric"} {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodPost, path, nil)
		mux.ServeHTTP(recorder, request)
		if recorder.Code != http.StatusMethodNotAllowed {
			t.Fatalf("POST %s status = %d, want %d", path, recorder.Code, http.StatusMethodNotAllowed)
		}
	}
}
