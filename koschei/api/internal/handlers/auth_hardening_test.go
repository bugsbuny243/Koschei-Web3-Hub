package handlers

import (
	"encoding/base64"
	"testing"
)

func TestSanitizeFrontendRedirectRejectsCrossOriginBackslashForms(t *testing.T) {
	tests := []string{
		`/\\audit.invalid`,
		`/foo\\bar`,
		"/safe\r\nLocation: https://audit.invalid",
		"//audit.invalid/path",
		"https://audit.invalid/path",
	}
	for _, input := range tests {
		if got := sanitizeFrontendRedirect(input); got != "" {
			t.Fatalf("sanitizeFrontendRedirect(%q) = %q; want rejection", input, got)
		}
	}
}

func TestSanitizeFrontendRedirectAllowsLocalPaths(t *testing.T) {
	for _, input := range []string{"/hub.html", "/case/abc?tab=evidence", "/"} {
		if got := sanitizeFrontendRedirect(input); got != input {
			t.Fatalf("sanitizeFrontendRedirect(%q) = %q; want %q", input, got, input)
		}
	}
}

func TestTryLocalJWTMalformedKidDoesNotPanic(t *testing.T) {
	headers := []string{
		`{}`,
		`{"kid":null}`,
		`{"kid":123}`,
		`{"kid":true}`,
	}
	for _, header := range headers {
		t.Run(header, func(t *testing.T) {
			token := base64.RawURLEncoding.EncodeToString([]byte(header)) + "." +
				base64.RawURLEncoding.EncodeToString([]byte(`{}`)) + "." +
				base64.RawURLEncoding.EncodeToString([]byte("sig"))

			defer func() {
				if recovered := recover(); recovered != nil {
					t.Fatalf("tryLocalJWT panicked for malformed kid: %v", recovered)
				}
			}()

			_, local, err := tryLocalJWT(token)
			if local {
				t.Fatalf("malformed kid must not be treated as local token; err=%v", err)
			}
		})
	}
}
