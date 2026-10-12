package radaradapter

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
)

type tronKeyTransport func(*http.Request) (*http.Response, error)

func (fn tronKeyTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}

func TestTronHeightReaderSendsConfiguredAPIKey(t *testing.T) {
	t.Setenv("TRONGRID_API_KEY", "test-tron-key")
	requests := 0
	client := &http.Client{Transport: tronKeyTransport(func(req *http.Request) (*http.Response, error) {
		requests++
		if req.URL.String() != "https://api.trongrid.io/wallet/getnowblock" ||
			req.Method != http.MethodPost || req.Header.Get("TRON-PRO-API-KEY") != "test-tron-key" {
			return nil, errors.New("unexpected TronGrid request or missing API key")
		}
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`{"block_header":{"raw_data":{"number":12345}}}`)), Header: make(http.Header)}, nil
	})}
	height, err := ReadTronMainnetHeight(context.Background(), client)
	if err != nil || height != 12345 || requests != 1 {
		t.Fatalf("height=%d requests=%d err=%v", height, requests, err)
	}
}

func TestTronHeightReaderRejectsProviderRedirect(t *testing.T) {
	t.Setenv("TRONGRID_API_KEY", "test-tron-key")
	requests := 0
	client := &http.Client{Transport: tronKeyTransport(func(req *http.Request) (*http.Response, error) {
		requests++
		return &http.Response{StatusCode: http.StatusFound, Header: http.Header{"Location": []string{"https://untrusted.example/"}}, Body: io.NopCloser(strings.NewReader(""))}, nil
	})}
	if _, err := ReadTronMainnetHeight(context.Background(), client); err == nil {
		t.Fatal("redirect must fail closed")
	}
	if requests != 1 {
		t.Fatalf("unexpected redirected request count: %d", requests)
	}
}
