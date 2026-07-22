package defense

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"time"
)

type LiteSVMWorkerRuntime struct {
	WorkerID                string
	WorkerImageDigest       string
	WorkerEnabled           bool
	SandboxEnabled          bool
	HarnessExecutionEnabled bool
	LiteSVMExecutionEnabled bool
	NetworkIsolated         bool
}

// ProcessWorkerJobWithRuntime preserves the legacy verification path and adds
// the Phase 12C action only when every worker-side gate is explicitly true.
func ProcessWorkerJobWithRuntime(ctx context.Context, db *sql.DB, job WorkerJob, runtime LiteSVMWorkerRuntime) (map[string]any, error) {
	if job.Action != WorkerActionRunLiteSVMHarness {
		return ProcessWorkerJob(ctx, db, job, runtime.SandboxEnabled)
	}
	attempt, err := ExecuteLiteSVMWorkerJob(ctx, db, job, runtime)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"action": job.Action,
		"attempt": attempt,
		"worker_execution": true,
		"network_access": false,
		"dependency_resolution": false,
		"wallet_material_accessed": false,
		"mainnet_rpc_accessed": false,
		"mainnet_transaction_sent": false,
		"verdict_authority": false,
	}, nil
}

// ExecuteLiteSVMWorkerJob is the only Phase 12C process-launch boundary. It
// launches the pinned Cargo executable directly, never through a shell.
func ExecuteLiteSVMWorkerJob(ctx context.Context, db *sql.DB, job WorkerJob, runtime LiteSVMWorkerRuntime) (LiteSVMExecutionAttempt, error) {
	if !runtime.WorkerEnabled || !runtime.SandboxEnabled || !runtime.HarnessExecutionEnabled || !runtime.LiteSVMExecutionEnabled {
		return LiteSVMExecutionAttempt{}, errors.New("Phase 12C worker execution gates are disabled")
	}
	if !runtime.NetworkIsolated {
		return LiteSVMExecutionAttempt{}, errors.New("isolated worker network boundary is not attested")
	}
	if strings.TrimSpace(runtime.WorkerID) == "" || normalizeDefenseSHA256Digest(runtime.WorkerImageDigest) == "" {
		return LiteSVMExecutionAttempt{}, errors.New("live worker identity or image digest is unavailable")
	}
	if job.Action != WorkerActionRunLiteSVMHarness || job.ProfileRef == "" || job.MaterializationRef == "" {
		return LiteSVMExecutionAttempt{}, errors.New("LiteSVM worker job is incomplete")
	}
	plan, err := PrepareLiteSVMExecution(ctx, db, job.JobRef, job.ProfileRef, job.MaterializationRef, runtime.WorkerID, runtime.WorkerImageDigest)
	if err != nil {
		return LiteSVMExecutionAttempt{}, err
	}

	startedAt := time.Now().UTC()
	inputRoot, err := os.MkdirTemp("", "koschei-litesvm-input-")
	if err != nil {
		return persistLiteSVMStartRejection(ctx, db, plan, job.Attempts, startedAt, "sandbox_unavailable", err)
	}
	defer removeLiteSVMTree(inputRoot)
	scratchRoot, err := os.MkdirTemp("", "koschei-litesvm-scratch-")
	if err != nil {
		return persistLiteSVMStartRejection(ctx, db, plan, job.Attempts, startedAt, "sandbox_unavailable", err)
	}
	defer removeLiteSVMTree(scratchRoot)
	if err := materializeBundle(inputRoot, plan.Bundle); err != nil {
		return persistLiteSVMStartRejection(ctx, db, plan, job.Attempts, startedAt, "materialized_artifact_mismatch", err)
	}
	if err := makeLiteSVMInputReadOnly(inputRoot); err != nil {
		return persistLiteSVMStartRejection(ctx, db, plan, job.Attempts, startedAt, "sandbox_unavailable", err)
	}
	environment, err := buildLiteSVMEnvironment(plan, scratchRoot)
	if err != nil {
		return persistLiteSVMStartRejection(ctx, db, plan, job.Attempts, startedAt, "sandbox_unavailable", err)
	}

	maxDuration := time.Duration(plan.MaxDurationSeconds) * time.Second
	runCtx, cancel := context.WithTimeout(ctx, maxDuration)
	defer cancel()
	stdout := newBoundedExecutionBuffer(plan.MaxOutputBytes)
	stderr := newBoundedExecutionBuffer(plan.MaxOutputBytes)
	cmd := exec.Command(plan.CargoExecutablePath, "test", "--locked", "--offline")
	cmd.Dir = inputRoot
	cmd.Env = environment
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := cmd.Start(); err != nil {
		return persistLiteSVMStartRejection(ctx, db, plan, job.Attempts, startedAt, "sandbox_unavailable", err)
	}

	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	status := "completed"
	terminationReason := "process_exited"
	var runErr error
	select {
	case runErr = <-done:
	case <-runCtx.Done():
		_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		runErr = <-done
		if errors.Is(ctx.Err(), context.Canceled) || errors.Is(ctx.Err(), context.DeadlineExceeded) {
			status = "cancelled"
			terminationReason = "execution_cancelled"
		} else {
			status = "timed_out"
			terminationReason = "execution_timeout"
		}
	}
	completedAt := time.Now().UTC()
	exitCode := 0
	if cmd.ProcessState != nil {
		exitCode = cmd.ProcessState.ExitCode()
	}
	if status == "completed" && runErr != nil {
		status = "failed"
		terminationReason = classifyLiteSVMProcessFailure(stdout.String(), stderr.String())
	}
	limitations := []string{
		"The worker reported a configured no-egress deployment boundary; application evidence alone cannot prove infrastructure isolation.",
		"A single deterministic harness result does not establish exploitability, reachability, asset impact, proof-of-fix or program safety.",
	}
	if stdout.Truncated() || stderr.Truncated() {
		limitations = append(limitations, "Process output exceeded the immutable profile bound and was truncated deterministically.")
	}
	attempt, persistErr := PersistLiteSVMExecutionAttempt(ctx, db, plan, LiteSVMExecutionOutcome{
		AttemptNumber: job.Attempts,
		Status: status,
		StartedAt: startedAt,
		CompletedAt: completedAt,
		ExitCode: &exitCode,
		TerminationReason: terminationReason,
		Stdout: stdout.String(),
		Stderr: stderr.String(),
		StdoutTruncated: stdout.Truncated(),
		StderrTruncated: stderr.Truncated(),
		SourceExecuted: true,
		HarnessExecuted: true,
		Limitations: limitations,
	})
	if persistErr != nil {
		return LiteSVMExecutionAttempt{}, persistErr
	}
	return attempt, nil
}

func persistLiteSVMStartRejection(ctx context.Context, db *sql.DB, plan LiteSVMExecutionPlan, attemptNumber int, startedAt time.Time, reason string, cause error) (LiteSVMExecutionAttempt, error) {
	limitations := []string{}
	if cause != nil {
		limitations = append(limitations, strings.TrimSpace(cause.Error()))
	}
	attempt, err := PersistLiteSVMExecutionAttempt(ctx, db, plan, LiteSVMExecutionOutcome{
		AttemptNumber: attemptNumber,
		Status: "rejected",
		StartedAt: startedAt,
		CompletedAt: time.Now().UTC(),
		TerminationReason: reason,
		SourceExecuted: false,
		HarnessExecuted: false,
		Limitations: limitations,
	})
	if err != nil {
		return LiteSVMExecutionAttempt{}, err
	}
	return attempt, nil
}

func buildLiteSVMEnvironment(plan LiteSVMExecutionPlan, scratchRoot string) ([]string, error) {
	scratchRoot = filepath.Clean(scratchRoot)
	if !filepath.IsAbs(scratchRoot) {
		return nil, errors.New("LiteSVM scratch root must be absolute")
	}
	directories := map[string]string{
		"HOME": filepath.Join(scratchRoot, "home"),
		"CARGO_HOME": filepath.Join(scratchRoot, "cargo-home"),
		"RUSTUP_HOME": filepath.Join(scratchRoot, "rustup-home"),
		"CARGO_TARGET_DIR": filepath.Join(scratchRoot, "target"),
		"TMPDIR": filepath.Join(scratchRoot, "tmp"),
	}
	for _, path := range directories {
		if err := os.MkdirAll(path, 0o700); err != nil {
			return nil, err
		}
	}
	rustcPath := ""
	toolDirs := []string{}
	for _, evidence := range plan.ExecutableEvidence {
		if evidence.BinaryPath == "" || !filepath.IsAbs(evidence.BinaryPath) {
			return nil, fmt.Errorf("pinned tool path is not absolute: %s", evidence.ToolName)
		}
		toolDirs = append(toolDirs, filepath.Dir(evidence.BinaryPath))
		if evidence.ToolName == "rustc" {
			rustcPath = evidence.BinaryPath
		}
	}
	toolDirs = uniqueStrings(toolDirs)
	sort.Strings(toolDirs)
	if rustcPath == "" || len(toolDirs) == 0 {
		return nil, errors.New("pinned Rust tool environment is incomplete")
	}
	env := map[string]string{
		"HOME": directories["HOME"],
		"CARGO_HOME": directories["CARGO_HOME"],
		"RUSTUP_HOME": directories["RUSTUP_HOME"],
		"CARGO_TARGET_DIR": directories["CARGO_TARGET_DIR"],
		"TMPDIR": directories["TMPDIR"],
		"PATH": strings.Join(toolDirs, string(os.PathListSeparator)),
		"RUSTC": rustcPath,
		"CARGO_NET_OFFLINE": "true",
		"CARGO_TERM_COLOR": "never",
		"GIT_CONFIG_NOSYSTEM": "1",
		"GIT_TERMINAL_PROMPT": "0",
		"KOSCHEI_DEFENSE_ISOLATED": "1",
		"LANG": "C.UTF-8",
		"LC_ALL": "C.UTF-8",
		"RUST_BACKTRACE": "0",
		"SOURCE_DATE_EPOCH": "0",
		"TERM": "dumb",
		"TZ": "UTC",
	}
	keys := make([]string, 0, len(env))
	for key := range env {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	out := make([]string, 0, len(keys))
	for _, key := range keys {
		out = append(out, key+"="+env[key])
	}
	return out, nil
}

func makeLiteSVMInputReadOnly(root string) error {
	return filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return os.Chmod(path, 0o500)
		}
		return os.Chmod(path, 0o400)
	})
}

func removeLiteSVMTree(root string) {
	_ = filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil || info == nil {
			return nil
		}
		if info.IsDir() {
			_ = os.Chmod(path, 0o700)
		} else {
			_ = os.Chmod(path, 0o600)
		}
		return nil
	})
	_ = os.RemoveAll(root)
}

func classifyLiteSVMProcessFailure(stdout, stderr string) string {
	combined := strings.ToLower(stdout + "\n" + stderr)
	patterns := []string{
		"no matching package named",
		"failed to download",
		"attempting to make an http request, but --offline was specified",
		"unable to update registry",
		"could not find",
	}
	for _, pattern := range patterns {
		if strings.Contains(combined, pattern) {
			return "dependency_unavailable_offline"
		}
	}
	return "process_failed"
}

type boundedExecutionBuffer struct {
	buffer    bytes.Buffer
	remaining int
	truncated bool
}

func newBoundedExecutionBuffer(maximum int) *boundedExecutionBuffer {
	if maximum < 16*1024 || maximum > 1024*1024 {
		maximum = 256 * 1024
	}
	return &boundedExecutionBuffer{remaining: maximum}
}

func (b *boundedExecutionBuffer) Write(p []byte) (int, error) {
	original := len(p)
	if b.remaining <= 0 {
		b.truncated = b.truncated || original > 0
		return original, nil
	}
	write := p
	if len(write) > b.remaining {
		write = write[:b.remaining]
		b.truncated = true
	}
	_, _ = b.buffer.Write(write)
	b.remaining -= len(write)
	return original, nil
}

func (b *boundedExecutionBuffer) String() string {
	return strings.ToValidUTF8(b.buffer.String(), "\uFFFD")
}

func (b *boundedExecutionBuffer) Truncated() bool { return b.truncated }
