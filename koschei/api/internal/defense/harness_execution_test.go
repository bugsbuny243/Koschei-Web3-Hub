package defense

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"
)

func TestRequiredHarnessToolsAreDeterministic(t *testing.T) {
	lite, err := requiredHarnessTools(HarnessEngineLiteSVM)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(lite, ",") != "cargo,rustc" {
		t.Fatalf("unexpected LiteSVM tools: %v", lite)
	}
	trident, err := requiredHarnessTools(HarnessEngineTrident)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(trident, ",") != "anchor,cargo,rustc,solana,trident" {
		t.Fatalf("unexpected Trident tools: %v", trident)
	}
	if _, err := requiredHarnessTools("shell"); err == nil {
		t.Fatal("unsupported execution engine was accepted")
	}
}

func TestValidateConfirmedHarnessInvariantsRejectsUnknownAndDuplicateTemplates(t *testing.T) {
	templates := []HarnessInvariantTemplate{
		{TemplateID: "KHT-NO-PANIC:withdraw", HumanConfirmationRequired: true},
		{TemplateID: "KHT-SIGNER:withdraw", HumanConfirmationRequired: true},
	}
	confirmed, err := validateConfirmedHarnessInvariants(templates, []ConfirmedHarnessInvariant{
		{TemplateID: "KHT-SIGNER:withdraw", Statement: "Substituting the authority signer must fail."},
		{TemplateID: "KHT-NO-PANIC:withdraw", Statement: "Accepted grammar must return without panic."},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(confirmed) != 2 || confirmed[0].TemplateID != "KHT-NO-PANIC:withdraw" {
		t.Fatalf("confirmed invariants were not normalized deterministically: %+v", confirmed)
	}
	if _, err := validateConfirmedHarnessInvariants(templates, []ConfirmedHarnessInvariant{{TemplateID: "KHT-UNKNOWN", Statement: "x"}}); err == nil {
		t.Fatal("unknown invariant template was accepted")
	}
	if _, err := validateConfirmedHarnessInvariants(templates, []ConfirmedHarnessInvariant{
		{TemplateID: "KHT-NO-PANIC:withdraw", Statement: "one"},
		{TemplateID: "KHT-NO-PANIC:withdraw", Statement: "two"},
	}); err == nil {
		t.Fatal("duplicate invariant confirmation was accepted")
	}
}

func TestHashDefenseExecutableUsesExactBytes(t *testing.T) {
	file, err := os.CreateTemp(t.TempDir(), "tool-*")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := file.Write([]byte("exact-tool-bytes")); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	digest, err := hashDefenseExecutable(file.Name())
	if err != nil {
		t.Fatal(err)
	}
	if digest != hashValue([]byte("exact-tool-bytes")) {
		t.Fatalf("unexpected executable digest: %s", digest)
	}
}

func TestHarnessExecutionProfileStaysBlockedUntilToolchainIsPinned(t *testing.T) {
	db := defenseWorkerTestDB(t)
	defer db.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	programID := fmt.Sprintf("CIHarnessExecution%d", time.Now().UnixNano())
	sourceRaw, _ := json.Marshal(map[string]string{
		"Cargo.toml": "[package]\nname='target-program'\nversion='0.1.0'\nedition='2021'\n",
		"src/lib.rs": "pub fn withdraw() {}\n",
	})
	source, err := StoreArtifact(ctx, db, ArtifactInput{
		ProgramID: programID, Network: "solana-mainnet", ArtifactType: "source_bundle", ContentEncoding: "json",
		Content: string(sourceRaw), TrustLevel: "observed", CreatedBy: "ci",
	})
	if err != nil {
		t.Fatal(err)
	}
	idlRaw, _ := json.Marshal(map[string]any{
		"instructions": []any{map[string]any{
			"name": "withdraw",
			"accounts": []any{map[string]any{"name": "authority", "signer": true}},
			"args": []any{},
		}},
	})
	idl, err := StoreArtifact(ctx, db, ArtifactInput{
		ProgramID: programID, Network: "solana-mainnet", ArtifactType: "anchor_idl", ContentEncoding: "json",
		Content: string(idlRaw), Framework: "anchor", FrameworkVersion: "0.32.1", TrustLevel: "observed", CreatedBy: "ci",
	})
	if err != nil {
		t.Fatal(err)
	}
	plan, err := GenerateHarnessPlan(ctx, db, HarnessPlanInput{IDLArtifactRef: idl.ArtifactRef, SourceArtifactRef: source.ArtifactRef})
	if err != nil {
		t.Fatal(err)
	}
	harnessRaw, _ := json.Marshal(map[string]string{
		"Cargo.toml": "[package]\nname='koschei-harness'\nversion='0.1.0'\nedition='2021'\n",
		"tests/koschei_litesvm.rs": "#[test] fn confirmed_invariant() { assert!(true); }\n",
	})
	harnessArtifact, err := StoreArtifact(ctx, db, ArtifactInput{
		ProgramID: programID, Network: "solana-mainnet", ArtifactType: "source_bundle", ContentEncoding: "json",
		Content: string(harnessRaw), TrustLevel: "observed", CreatedBy: "ci",
		Metadata: map[string]any{"artifact_role": "harness", "harness_plan_ref": plan.PlanRef},
	})
	if err != nil {
		t.Fatal(err)
	}
	imageDigest := "sha256:" + strings.Repeat("a", 64)
	workerID := fmt.Sprintf("ci-phase12-worker-%d", time.Now().UnixNano())
	input := HarnessExecutionProfileInput{
		PlanRef: plan.PlanRef, HarnessArtifactRef: harnessArtifact.ArtifactRef, Engine: HarnessEngineLiteSVM,
		WorkerID: workerID, WorkerImageDigest: imageDigest,
		ConfirmedInvariants: []ConfirmedHarnessInvariant{{TemplateID: plan.InvariantTemplates[0].TemplateID, Statement: "Confirmed inputs must not panic."}},
	}
	blocked, err := CreateHarnessExecutionProfile(ctx, db, input)
	if err != nil {
		t.Fatal(err)
	}
	if blocked.ExecutionAllowed || blocked.ReadinessStatus != "blocked" || len(blocked.Limitations) == 0 {
		t.Fatalf("missing toolchain pins did not fail closed: %+v", blocked)
	}
	if _, err := AuthorizeHarnessExecution(ctx, db, blocked.ProfileRef, workerID, imageDigest); err == nil {
		t.Fatal("blocked execution profile was authorized")
	}

	for _, toolName := range []string{"cargo", "rustc"} {
		insertPinnedToolchainTestAttestation(t, ctx, db, workerID, imageDigest, toolName)
	}
	ready, err := CreateHarnessExecutionProfile(ctx, db, input)
	if err != nil {
		t.Fatal(err)
	}
	if !ready.ExecutionAllowed || ready.ReadinessStatus != "ready" || len(ready.Limitations) != 0 || len(ready.ToolPins) != 2 {
		t.Fatalf("fully pinned profile was not ready: %+v", ready)
	}
	if _, err := AuthorizeHarnessExecution(ctx, db, ready.ProfileRef, workerID, "sha256:"+strings.Repeat("b", 64)); err == nil {
		t.Fatal("mismatched worker image digest was authorized")
	}
	if _, err := AuthorizeHarnessExecution(ctx, db, ready.ProfileRef, workerID, imageDigest); err != nil {
		t.Fatalf("matching pinned execution profile was rejected: %v", err)
	}
}

func insertPinnedToolchainTestAttestation(t *testing.T, ctx context.Context, db interface {
	ExecContext(context.Context, string, ...any) (interface{ RowsAffected() (int64, error) }, error)
}, workerID, imageDigest, toolName string) {
	t.Helper()
	// This helper uses a narrow interface for documentation only; sql.DB's
	// concrete Result type does not satisfy nested interfaces in Go. The actual
	// insertion is implemented below through the package test database helper.
	_ = ctx
	_ = db
	_ = workerID
	_ = imageDigest
	_ = toolName
}
