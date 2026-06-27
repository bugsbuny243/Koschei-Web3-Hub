package webhooks

import (
	"context"
	"net"
	"testing"
)

func TestIsPublicIP(t *testing.T) {
	cases := []struct {
		value string
		want  bool
	}{
		{"127.0.0.1", false},
		{"10.0.0.1", false},
		{"172.16.0.1", false},
		{"192.168.1.1", false},
		{"169.254.169.254", false},
		{"100.64.0.1", false},
		{"8.8.8.8", true},
		{"1.1.1.1", true},
		{"::1", false},
		{"fd00::1", false},
		{"2606:4700:4700::1111", true},
	}
	for _, item := range cases {
		if got := isPublicIP(net.ParseIP(item.value)); got != item.want {
			t.Fatalf("isPublicIP(%s)=%v want %v", item.value, got, item.want)
		}
	}
}

func TestValidateEndpointURLRejectsUnsafeInputs(t *testing.T) {
	t.Setenv("APP_ENV", "production")
	for _, raw := range []string{
		"http://example.com/hook",
		"https://localhost/hook",
		"https://service.local/hook",
		"https://127.0.0.1/hook",
		"https://169.254.169.254/latest/meta-data",
		"https://user:pass@example.com/hook",
	} {
		if _, err := ValidateEndpointURL(context.Background(), raw); err == nil {
			t.Fatalf("expected %q to be rejected", raw)
		}
	}
}
