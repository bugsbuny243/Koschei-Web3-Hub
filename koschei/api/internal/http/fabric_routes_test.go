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
	if snapshot.NativeSchemaMutation {
		t.Fatal("Fabric must not mutate existing native v1 schemas")
	}
	if snapshot.SharedEnvelopeSchema != "fabric.security-case-envelope.v1" {
		t.Fatalf("shared envelope = %q, want fabric.security-case-envelope.v1", snapshot.SharedEnvelopeSchema)
	}
	if len(snapshot.P0Blockers) != 8 {
		t.Fatalf("P0 blockers = %d, want 8", len(snapshot.P0Blockers))
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

func TestFabricPackageSequenceStartsWithABAndCTracking(t *testing.T) {
	snapshot := currentFabricSnapshot()
	if len(snapshot.PackageStates) != 8 {
		t.Fatalf("package states = %d, want 8", len(snapshot.PackageStates))
	}
	if snapshot.PackageStates[0].ID != "A" || snapshot.PackageStates[0].State != "in-progress" {
		t.Fatalf("package A = %#v, want in-progress", snapshot.PackageStates[0])
	}
	if snapshot.PackageStates[1].ID != "B" || snapshot.PackageStates[1].State != "blocked" {
		t.Fatalf("package B = %#v, want blocked", snapshot.PackageStates[1])
	}
	if snapshot.PackageStates[2].ID != "C" || snapshot.PackageStates[2].State != "in-progress" {
		t.Fatalf("package C = %#v, want in-progress", snapshot.PackageStates[2])
	}
}

func TestFabricCapabilityAPI(t *testing.T) {
	mux := http.NewServeMux()
	registerFabricRoutes(mux)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/fabric/capabilities", nil)
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
	if snapshot.AcceptanceCaseCount != 14 {
		t.Fatalf("acceptanceCaseCount = %d, want 14", snapshot.AcceptanceCaseCount)
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
	for _, needle := range []string{
		"Koschei Fabric",
		"koschei-web3",
		"koschei-lang",
		"koschei-sentinel",
		"fabric.security-case-envelope.v1",
		"Security work sequence",
		"CORE-01",
		"SUPPLY-02",
	} {
		if !strings.Contains(body, needle) {
			t.Fatalf("operator surface missing %q", needle)
		}
	}
}

func TestFabricRoutesRejectMutationMethods(t *testing.T) {
	mux := http.NewServeMux()
	registerFabricRoutes(mux)

	for _, path := range []string{"/fabric/capabilities", "/fabric"} {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodPost, path, nil)
		mux.ServeHTTP(recorder, request)
		if recorder.Code != http.StatusMethodNotAllowed {
			t.Fatalf("POST %s status = %d, want %d", path, recorder.Code, http.StatusMethodNotAllowed)
		}
	}
}

func TestMountFabricPreservesExistingRouteAndSecuresFabric(t *testing.T) {
	base := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Legacy-Handler", "preserved")
		_, _ = w.Write([]byte("legacy:" + r.URL.Path))
	})
	mounted := MountFabric(base)

	legacy := httptest.NewRecorder()
	mounted.ServeHTTP(legacy, httptest.NewRequest(http.MethodGet, "/health", nil))
	if legacy.Header().Get("X-Legacy-Handler") != "preserved" || legacy.Body.String() != "legacy:/health" {
		t.Fatalf("legacy route was not delegated unchanged: headers=%v body=%q", legacy.Header(), legacy.Body.String())
	}

	fabric := httptest.NewRecorder()
	mounted.ServeHTTP(fabric, httptest.NewRequest(http.MethodGet, "/fabric/capabilities", nil))
	if fabric.Code != http.StatusOK {
		t.Fatalf("Fabric status = %d, want %d", fabric.Code, http.StatusOK)
	}
	if fabric.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Fatal("Fabric response did not pass through security headers")
	}
	if fabric.Header().Get("Content-Security-Policy") == "" {
		t.Fatal("Fabric response is missing Content-Security-Policy")
	}
}
