package services

import (
	"os"
	"strings"
	"testing"
)

func TestArvisStreamEventKeepsSourceSignatureSeparateFromVerdictAuthentication(t *testing.T) {
	source, err := os.ReadFile("arvis_stream_verdict_worker.go")
	if err != nil {
		t.Fatal(err)
	}
	text := string(source)
	if strings.Contains(text, "Signature: arm.Signature, SourceAddress: target.ProgramID") {
		t.Fatal("stream event identity must not reuse the cryptographic arm verdict signature")
	}
	if !strings.Contains(text, "Signature: target.Signature, SourceAddress: target.ProgramID") {
		t.Fatal("stream event persistence must retain the source stream/Solana signature")
	}
}
