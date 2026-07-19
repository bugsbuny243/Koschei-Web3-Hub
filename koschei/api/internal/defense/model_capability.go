package defense

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"time"

	"koschei/api/internal/router"
)

var allowedModelCapabilityRoles = map[string]bool{
	"general": true,
	"lead_prosecutor": true,
	"evidence_prosecutor": true,
	"tribunal_qwen": true,
	"tribunal_glm": true,
	"defense_engineer": true,
	"custom": true,
}

type ModelCapabilityInput struct {
	Model     string `json:"model"`
	ModelRole string `json:"model_role"`
}

type ModelCapabilitySnapshot struct {
	CapabilityRef             string         `json:"capability_ref"`
	Provider                  string         `json:"provider"`
	Model                     string         `json:"model"`
	ModelRole                 string         `json:"model_role"`
	Endpoint                  string         `json:"endpoint"`
	Available                 bool           `json:"available"`
	StructuredOutputSupported bool           `json:"structured_output_supported"`
	ToolCallingSupported      bool           `json:"tool_calling_supported"`
	BasicLatencyMS            int            `json:"basic_latency_ms"`
	StructuredLatencyMS       int            `json:"structured_latency_ms"`
	ToolLatencyMS             int            `json:"tool_latency_ms"`
	Status                    string         `json:"status"`
	BasicResult               map[string]any `json:"basic_result"`
	StructuredResult          map[string]any `json:"structured_result"`
	ToolResult                map[string]any `json:"tool_result"`
	Limitations               []string       `json:"limitations"`
	CapabilityHash            string         `json:"capability_hash"`
	VerdictAuthority          bool           `json:"verdict_authority"`
	ObservedAt                time.Time      `json:"observed_at"`
	CreatedBy                 string         `json:"created_by"`
}

func ProbeAndPersistModelCapability(ctx context.Context, db *sql.DB, input ModelCapabilityInput) (ModelCapabilitySnapshot, error) {
	input.ModelRole = normalizeModelCapabilityRole(input.ModelRole)
	if !allowedModelCapabilityRoles[input.ModelRole] {
		return ModelCapabilitySnapshot{}, errors.New("unsupported model capability role")
	}
	input.Model = strings.TrimSpace(input.Model)
	if input.Model == "" {
		input.Model = ResolveTogetherModelForRole(input.ModelRole)
	}
	if input.Model == "" {
		return ModelCapabilitySnapshot{}, errors.New("no Together model is configured for this role")
	}
	result, err := router.ProbeTogetherCapabilities(ctx, router.TogetherCapabilityProbeRequest{
		Model: input.Model,
		APIKey: strings.TrimSpace(os.Getenv("TOGETHER_API_KEY")),
		Endpoint: strings.TrimSpace(os.Getenv("TOGETHER_BASE_URL")),
		Timeout: 45 * time.Second,
	})
	if err != nil {
		return ModelCapabilitySnapshot{}, err
	}
	return PersistModelCapabilitySnapshot(ctx, db, input.ModelRole, result)
}

func PersistModelCapabilitySnapshot(ctx context.Context, db *sql.DB, role string, result router.TogetherCapabilityProbeResult) (ModelCapabilitySnapshot, error) {
	if db == nil {
		return ModelCapabilitySnapshot{}, errors.New("database unavailable")
	}
	role = normalizeModelCapabilityRole(role)
	if !allowedModelCapabilityRoles[role] {
		return ModelCapabilitySnapshot{}, errors.New("unsupported model capability role")
	}
	status := modelCapabilityStatus(result)
	limitations := modelCapabilityLimitations(result, status)
	observedAt := result.ObservedAt.UTC()
	if observedAt.IsZero() {
		observedAt = time.Now().UTC()
	}
	basicResult := capabilityStepMap(result.Basic)
	structuredResult := capabilityStepMap(result.Structured)
	toolResult := capabilityStepMap(result.Tool)
	payload := map[string]any{
		"provider": "together",
		"model": result.Model,
		"model_role": role,
		"endpoint": result.Endpoint,
		"available": result.Available,
		"structured_output_supported": result.StructuredOutputSupported,
		"tool_calling_supported": result.ToolCallingSupported,
		"basic_latency_ms": result.Basic.LatencyMS,
		"structured_latency_ms": result.Structured.LatencyMS,
		"tool_latency_ms": result.Tool.LatencyMS,
		"status": status,
		"basic_result": basicResult,
		"structured_result": structuredResult,
		"tool_result": toolResult,
		"limitations": limitations,
		"observed_at": observedAt.Format(time.RFC3339Nano),
	}
	capabilityHash := hashJSON(payload)
	capabilityRef := prefixedID("KMC1-", payload)
	basicRaw, _ := json.Marshal(basicResult)
	structuredRaw, _ := json.Marshal(structuredResult)
	toolRaw, _ := json.Marshal(toolResult)
	limitationsRaw, _ := json.Marshal(limitations)
	_, err := db.ExecContext(ctx, `INSERT INTO defense_model_capability_snapshots
		(capability_ref,provider,model,model_role,endpoint,available,structured_output_supported,tool_calling_supported,
		 basic_latency_ms,structured_latency_ms,tool_latency_ms,status,basic_result,structured_result,tool_result,limitations,
		 capability_hash,verdict_authority,observed_at,created_by)
		VALUES($1,'together',$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12::jsonb,$13::jsonb,$14::jsonb,$15::jsonb,$16,false,$17,'owner')
		ON CONFLICT(capability_ref) DO NOTHING`, capabilityRef, result.Model, role, result.Endpoint, result.Available,
		result.StructuredOutputSupported, result.ToolCallingSupported, result.Basic.LatencyMS, result.Structured.LatencyMS,
		result.Tool.LatencyMS, status, string(basicRaw), string(structuredRaw), string(toolRaw), string(limitationsRaw), capabilityHash, observedAt)
	if err != nil {
		return ModelCapabilitySnapshot{}, err
	}
	return ModelCapabilitySnapshot{
		CapabilityRef: capabilityRef, Provider: "together", Model: result.Model, ModelRole: role, Endpoint: result.Endpoint,
		Available: result.Available, StructuredOutputSupported: result.StructuredOutputSupported, ToolCallingSupported: result.ToolCallingSupported,
		BasicLatencyMS: result.Basic.LatencyMS, StructuredLatencyMS: result.Structured.LatencyMS, ToolLatencyMS: result.Tool.LatencyMS,
		Status: status, BasicResult: basicResult, StructuredResult: structuredResult, ToolResult: toolResult,
		Limitations: limitations, CapabilityHash: capabilityHash, VerdictAuthority: false, ObservedAt: observedAt, CreatedBy: "owner",
	}, nil
}

func modelCapabilityStatus(result router.TogetherCapabilityProbeResult) string {
	if !result.Available {
		return "failed"
	}
	if !result.StructuredOutputSupported || !result.ToolCallingSupported {
		return "partial"
	}
	return "passed"
}

func modelCapabilityLimitations(result router.TogetherCapabilityProbeResult, status string) []string {
	out := []string{}
	if result.Basic.Error != "" {
		out = append(out, "Basic chat probe: "+result.Basic.Error)
	}
	if result.Structured.Error != "" {
		out = append(out, "Structured-output probe: "+result.Structured.Error)
	}
	if result.Tool.Error != "" {
		out = append(out, "Function-calling probe: "+result.Tool.Error)
	}
	if status != "passed" {
		out = append(out, "This model must not be routed to capabilities that did not pass the live probe.")
	}
	return uniqueStrings(out)
}

func capabilityStepMap(step router.CapabilityProbeStep) map[string]any {
	out := map[string]any{
		"supported": step.Supported,
		"status_code": step.StatusCode,
		"latency_ms": step.LatencyMS,
		"result": step.Result,
	}
	if step.Error != "" {
		out["error"] = step.Error
	}
	return out
}
