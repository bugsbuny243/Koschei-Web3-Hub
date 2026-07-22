package defense

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestBuildLiteSVMEnvironmentDoesNotInheritSecretsOrNetworkSettings(t *testing.T) {
	t.Setenv("TOGETHER_API_KEY", "secret")
	t.Setenv("SOLANA_RPC_URL", "https://rpc.invalid")
	t.Setenv("HTTPS_PROXY", "https://proxy.invalid")
	toolDir := t.TempDir()
	cargo := filepath.Join(toolDir, "cargo")
	rustc := filepath.Join(toolDir, "rustc")
	if err := os.WriteFile(cargo, []byte("cargo"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(rustc, []byte("rustc"), 0o755); err != nil {
		t.Fatal(err)
	}
	plan := LiteSVMExecutionPlan{ExecutableEvidence: []LiteSVMExecutableEvidence{
		{ToolName: "cargo", BinaryPath: cargo},
		{ToolName: "rustc", BinaryPath: rustc},
	}}
	scratch := t.TempDir()
	env, err := buildLiteSVMEnvironment(plan, scratch)
	if err != nil {
		t.Fatal(err)
	}
	joined := "\n" + strings.Join(env, "\n") + "\n"
	for _, forbidden := range []string{"TOGETHER_API_KEY=", "SOLANA_RPC_URL=", "HTTPS_PROXY=", "HTTP_PROXY=", "ALL_PROXY="} {
		if strings.Contains(joined, "\n"+forbidden) {
			t.Fatalf("isolated environment inherited forbidden variable %s: %v", forbidden, env)
		}
	}
	for _, required := range []string{"CARGO_NET_OFFLINE=true", "KOSCHEI_DEFENSE_ISOLATED=1", "TZ=UTC", "SOURCE_DATE_EPOCH=0", "RUSTC=" + rustc} {
		if !strings.Contains(joined, "\n"+required+"\n") {
			t.Fatalf("isolated environment is missing %s: %v", required, env)
		}
	}
	for _, dir := range []string{"home", "cargo-home", "rustup-home", "target", "tmp"} {
		info, err := os.Stat(filepath.Join(scratch, dir))
		if err != nil || !info.IsDir() {
			t.Fatalf("bounded scratch directory %s was not created: %v", dir, err)
		}
	}
}

func TestProcessWorkerJobWithRuntimeFailsClosedBeforeLiteSVMPreparation(t *testing.T) {
	job := WorkerJob{Action: WorkerActionRunLiteSVMHarness, ProfileRef: "KHEP1-example", MaterializationRef: "KHM1-example"}
	base := LiteSVMWorkerRuntime{
		WorkerID: "worker",
		WorkerImageDigest: "sha256:" + strings.Repeat("a", 64),
		WorkerEnabled: true,
		SandboxEnabled: true,
		HarnessExecutionEnabled: true,
		LiteSVMExecutionEnabled: true,
		NetworkIsolated: true,
	}
	cases := []struct {
		name string
		mutate func(*LiteSVMWorkerRuntime)
		want string
	}{
		{name: "worker disabled", mutate: func(v *LiteSVMWorkerRuntime) { v.WorkerEnabled = false }, want: "gates are disabled"},
		{name: "sandbox disabled", mutate: func(v *LiteSVMWorkerRuntime) { v.SandboxEnabled = false }, want: "gates are disabled"},
		{name: "harness gate disabled", mutate: func(v *LiteSVMWorkerRuntime) { v.HarnessExecutionEnabled = false }, want: "gates are disabled"},
		{name: "litesvm gate disabled", mutate: func(v *LiteSVMWorkerRuntime) { v.LiteSVMExecutionEnabled = false }, want: "gates are disabled"},
		{name: "network not isolated", mutate: func(v *LiteSVMWorkerRuntime) { v.NetworkIsolated = false }, want: "network boundary"},
		{name: "invalid image", mutate: func(v *LiteSVMWorkerRuntime) { v.WorkerImageDigest = "latest" }, want: "identity or image"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			runtime := base
			tc.mutate(&runtime)
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			if _, err := ProcessWorkerJobWithRuntime(ctx, nil, job, runtime); err == nil || !strings.Contains(strings.ToLower(err.Error()), strings.ToLower(tc.want)) {
				t.Fatalf("expected fail-closed error containing %q, got %v", tc.want, err)
			}
		})
	}
}

func TestLiteSVMFailureClassificationIsConservative(t *testing.T) {
	dependencyCases := []string{
		"no matching package named `litesvm` found",
		"attempting to make an HTTP request, but --offline was specified",
		"failed to download crate",
	}
	for _, text := range dependencyCases {
		if got := classifyLiteSVMProcessFailure("", text); got != "dependency_unavailable_offline" {
			t.Fatalf("dependency failure was misclassified: %q => %s", text, got)
		}
	}
	if got := classifyLiteSVMProcessFailure("test failed", "assertion failed"); got != "process_failed" {
		t.Fatalf("program/test failure was misclassified: %s", got)
	}
}

func TestBoundedExecutionBufferConsumesFullWritesAndCapsStoredBytes(t *testing.T) {
	buffer := newBoundedExecutionBuffer(16 * 1024)
	payload := strings.Repeat("x", 20*1024)
	n, err := buffer.Write([]byte(payload))
	if err != nil || n != len(payload) {
		t.Fatalf("bounded writer violated io.Writer contract: n=%d err=%v", n, err)
	}
	if !buffer.Truncated() || len(buffer.String()) != 16*1024 {
		t.Fatalf("bounded writer did not cap output: stored=%d truncated=%t", len(buffer.String()), buffer.Truncated())
	}
}
