package http

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestPermanentScanPathServesPublicScanner(t *testing.T) {
	staticDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(staticDir, "index.html"), []byte("index"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(staticDir, "scan.html"), []byte("public scan"), 0o644); err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(NewServer(nil, "", "", "", staticDir))
	defer srv.Close()
	resp, err := http.Get(srv.URL + "/scan/11111111111111111111111111111111")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK || string(body) != "public scan" {
		t.Fatalf("status=%d body=%q", resp.StatusCode, string(body))
	}
}
