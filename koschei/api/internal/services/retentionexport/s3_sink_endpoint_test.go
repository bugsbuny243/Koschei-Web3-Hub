package retentionexport

import "testing"

func TestNewS3SinkAcceptsEndpointWithoutScheme(t *testing.T) {
	base := S3Config{Bucket: "fixture-bucket", AccessKeyID: "fixture-id", SecretAccessKey: "fixture-secret"}
	for raw, want := range map[string]string{
		"account.r2.cloudflarestorage.com":    "https://account.r2.cloudflarestorage.com",
		" \"s3.eu-central-1.amazonaws.com\" ": "https://s3.eu-central-1.amazonaws.com",
		"storage.example.invalid:9000":        "https://storage.example.invalid:9000",
		"https://t3.storageapi.dev":           "https://t3.storageapi.dev",
		"http://minio.internal:9000":          "http://minio.internal:9000",
	} {
		config := base
		config.Endpoint = raw
		sink, err := NewS3Sink(config)
		if err != nil {
			t.Fatalf("endpoint %q must be accepted: %v", raw, err)
		}
		if got := sink.endpoint.String(); got != want {
			t.Fatalf("endpoint %q normalized to %q, want %q", raw, got, want)
		}
	}
	for _, raw := range []string{"", "   ", `""`, "ftp://storage.example.invalid"} {
		config := base
		config.Endpoint = raw
		if _, err := NewS3Sink(config); err == nil {
			t.Fatalf("endpoint %q must still be rejected", raw)
		}
	}
}
