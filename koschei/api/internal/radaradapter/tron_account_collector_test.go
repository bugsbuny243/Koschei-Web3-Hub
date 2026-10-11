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
	if _, err := observation.EvidenceRows(time.Now(), 0); err == nil {
		t.Fatal("accepted evidence without block height")
	}
	if observation.Network != "tron-mainnet" || len(observation.RawResponseSHA256) != 64 ||
		len(observation.Permissions) != 1 || !observation.Permissions[0].ThresholdReachable {
		t.Fatalf("unexpected observation: %+v", observation)
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
