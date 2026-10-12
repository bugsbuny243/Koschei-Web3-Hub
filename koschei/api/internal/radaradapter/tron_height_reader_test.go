package radaradapter

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestReadTronMainnetHeight(t *testing.T) {
	client := &http.Client{Transport: tronAccountRoundTrip(func(req *http.Request) (*http.Response, error) {
		if req.URL.String() != "https://api.trongrid.io/wallet/getnowblock" || req.Method != http.MethodPost {
			t.Fatalf("unexpected block request: %s %s", req.Method, req.URL)
		}
		return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header),
			Body: io.NopCloser(strings.NewReader(`{"block_header":{"raw_data":{"number":12345}}}`))}, nil
	})}
	height, err := ReadTronMainnetHeight(context.Background(), client)
	if err != nil || height != 12345 {
		t.Fatalf("unexpected Tron height: %d %v", height, err)
	}
}

func TestReadTronMainnetHeightFailsClosed(t *testing.T) {
	for _, tc := range []struct {
		name string
		status int
		body string
	}{
		{"HTTP failure", 429, "{}"},
		{"missing height", 200, "{}"},
		{"negative height", 200, `{"block_header":{"raw_data":{"number":-1}}}`},
		{"malformed", 200, "{"},
		{"oversized", 200, strings.Repeat("x", (1<<20)+1)},
		{"redirect", 302, "{}"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			client := &http.Client{Transport: tronAccountRoundTrip(func(*http.Request) (*http.Response, error) {
				return &http.Response{StatusCode: tc.status, Header: make(http.Header),
					Body: io.NopCloser(strings.NewReader(tc.body))}, nil
			})}
			if _, err := ReadTronMainnetHeight(context.Background(), client); err == nil {
				t.Fatal("accepted invalid Tron block response")
			}
		})
	}
}
