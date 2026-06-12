package handlers

import (
	"net/http"
	"strings"
	"testing"
)

const testJWT = "aaa.bbb.ccc"

func TestExtractAuthTokenChecksHeadersAndJSONPaths(t *testing.T) {
	tests := []struct {
		name   string
		header string
		body   string
	}{
		{name: "set auth jwt header", header: testJWT, body: `{}`},
		{name: "authorization bearer header", header: "Bearer " + testJWT, body: `{}`},
		{name: "x auth token header", header: testJWT, body: `{}`},
		{name: "top level token", body: `{"token":"` + testJWT + `"}`},
		{name: "top level jwt", body: `{"jwt":"` + testJWT + `"}`},
		{name: "top level access token", body: `{"access_token":"` + testJWT + `"}`},
		{name: "top level id token", body: `{"id_token":"` + testJWT + `"}`},
		{name: "top level auth token", body: `{"auth_token":"` + testJWT + `"}`},
		{name: "nested data token", body: `{"data":{"token":"` + testJWT + `"}}`},
		{name: "nested data jwt", body: `{"data":{"jwt":"` + testJWT + `"}}`},
		{name: "nested data access token", body: `{"data":{"access_token":"` + testJWT + `"}}`},
		{name: "nested data id token", body: `{"data":{"id_token":"` + testJWT + `"}}`},
		{name: "nested session token", body: `{"session":{"token":"` + testJWT + `"}}`},
		{name: "nested session jwt", body: `{"session":{"jwt":"` + testJWT + `"}}`},
		{name: "nested session access token", body: `{"session":{"access_token":"` + testJWT + `"}}`},
		{name: "nested session id token", body: `{"session":{"id_token":"` + testJWT + `"}}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := &http.Response{Header: http.Header{}}
			if tt.header != "" {
				if strings.Contains(tt.name, "authorization") {
					resp.Header.Set("Authorization", tt.header)
				} else if strings.Contains(tt.name, "x auth") {
					resp.Header.Set("x-auth-token", tt.header)
				} else {
					resp.Header.Set("set-auth-jwt", tt.header)
				}
			}
			token, ok := extractAuthToken(resp, []byte(tt.body))
			if !ok || token != testJWT {
				t.Fatalf("extractAuthToken() = %q, %t; want %q, true", token, ok, testJWT)
			}
		})
	}
}

func TestExtractAuthTokenIgnoresSessionCookieValues(t *testing.T) {
	resp := &http.Response{Header: http.Header{"Set-Cookie": []string{"session=secret.jwt.value; Path=/; HttpOnly"}}}
	token, ok := extractAuthToken(resp, []byte(`{"ok":true}`))
	if ok || token != "" {
		t.Fatalf("extractAuthToken() = %q, %t; want empty, false", token, ok)
	}
	if got := cookieNames(resp.Cookies()); len(got) != 1 || got[0] != "session" {
		t.Fatalf("cookieNames() = %#v, want session name only", got)
	}
}
