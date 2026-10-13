package radaradapter

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

type tronAccountRoundTrip func(*http.Request) (*http.Response, error)

func (fn tronAccountRoundTrip) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}

func TestCollectTronMainnetPermissions(t *testing.T) {
	const account = "41aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	client := &http.Client{Transport: tronAccountRoundTrip(func(req *http.Request) (*http.Response, error) {
		if req.URL.String() != "https://api.trongrid.io/wallet/getaccount" || req.Method != http.MethodPost {
			t.Fatalf("unexpected Tron request: %s %s", req.Method, req.URL)
		}
		body, err := io.ReadAll(req.Body)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(body), account) || !strings.Contains(string(body), `"visible":false`) {
			t.Fatalf("unexpected getaccount request: %s", body)
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(
			`{"address":"` + account + `","owner_permission":{"id":0,"type":0,"threshold":1,"keys":[{"address":"` + account + `","weight":1}]}}`,
		)), Header: make(http.Header)}, nil
	})}
	observation, err := CollectTronMainnetPermissions(context.Background(), client, account)
	if err != nil {
		t.Fatal(err)
	}
	rows, err := observation.EvidenceRows(time.Now(), 12345)
	if err != nil || len(rows) != 1 || rows[0].Status != "observed" || rows[0].Height != "12345" {
		t.Fatalf("unexpected evidence rows: %+v, %v", rows, err)
	}
	if rows[0].Attributes["account_state_height_verified"] != false {
		t.Fatal("unanchored account state incorrectly presented as verified")
	}
	observation.RawResponseSHA256 = strings.Repeat("z", 64)
	if _, err := observation.EvidenceRows(time.Now(), 12345); err == nil {
		t.Fatal("accepted non-hex raw response digest")
	}
	if _, err := observation.EvidenceRows(time.Now(), 0); err == nil {
		t.Fatal("accepted evidence without block height")
	}
	if observation.Network != "tron-mainnet" || len(observation.RawResponseSHA256) != 64 ||
		len(observation.Permissions) != 1 || !observation.Permissions[0].ThresholdReachable {
		t.Fatalf("unexpected observation: %+v", observation)
	}
}

func TestCollectTronMainnetPermissionsSendsConfiguredAPIKey(t *testing.T) {
	t.Setenv("TRONGRID_API_KEY", "test-tron-key")
	const account = "41aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	requests := 0
	client := &http.Client{Transport: tronAccountRoundTrip(func(req *http.Request) (*http.Response, error) {
		requests++
		if req.Header.Get("TRON-PRO-API-KEY") != "test-tron-key" {
			t.Fatal("TronGrid getaccount request omitted configured API key")
		}
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(
			`{"address":"` + account + `","owner_permission":{"id":0,"type":0,"threshold":1,"keys":[{"address":"` + account + `","weight":1}]}}`,
		)), Header: make(http.Header)}, nil
	})}
	observation, err := CollectTronMainnetPermissions(context.Background(), client, account)
	if err != nil || len(observation.Permissions) != 1 || requests != 1 {
		t.Fatalf("permissions=%d requests=%d err=%v", len(observation.Permissions), requests, err)
	}
}

func TestCollectTronMainnetPermissionsFailsClosed(t *testing.T) {
	const account = "41aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	for _, tc := range []struct {
		name string
		code int
		body string
	}{
		{name: "provider failure", code: 429, body: `{}`},
		{name: "missing account identity", code: 200, body: `{"owner_permission":{"id":0,"threshold":1}}`},
		{name: "wrong account identity", code: 200, body: `{"address":"41bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb","owner_permission":{"id":0,"threshold":1}}`},
		{name: "no permissions", code: 200, body: `{"address":"` + account + `"}`},
		{name: "invalid JSON", code: 200, body: `{"address":`},
		{name: "oversized response", code: 200, body: strings.Repeat("x", (1<<20)+1)},
		{name: "invalid signer", code: 200, body: `{"address":"` + account + `","owner_permission":{"id":0,"threshold":1,"keys":[{"address":"not-tron","weight":1}]}}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			client := &http.Client{Transport: tronAccountRoundTrip(func(*http.Request) (*http.Response, error) {
				return &http.Response{StatusCode: tc.code, Body: io.NopCloser(strings.NewReader(tc.body)), Header: make(http.Header)}, nil
			})}
			if _, err := CollectTronMainnetPermissions(context.Background(), client, account); err == nil {
				t.Fatal("accepted invalid or incomplete Tron provider evidence")
			}
		})
	}
	if _, err := CollectTronMainnetPermissions(context.Background(), nil, "invalid"); err == nil {
		t.Fatal("accepted invalid Tron target")
	}
}

func TestCollectTronMainnetPermissionsRejectsRedirect(t *testing.T) {
	const account = "41aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	calls := 0
	client := &http.Client{Transport: tronAccountRoundTrip(func(req *http.Request) (*http.Response, error) {
		calls++
		if calls > 1 {
			t.Fatal("followed Tron provider redirect")
		}
		response := &http.Response{StatusCode: http.StatusFound, Header: make(http.Header),
			Body: io.NopCloser(strings.NewReader(""))}
		response.Header.Set("Location", "https://example.com/collect")
		return response, nil
	})}
	if _, err := CollectTronMainnetPermissions(context.Background(), client, account); err == nil {
		t.Fatal("accepted redirected Tron provider response")
	}
	if calls != 1 {
		t.Fatalf("expected one provider request, got %d", calls)
	}
}
