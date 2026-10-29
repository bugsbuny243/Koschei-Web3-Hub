package openapi

import "strings"

func socOperation(route Route, method string) map[string]any {
	parameter := func(name string, required bool, schema map[string]any) any {
		return map[string]any{"name": name, "in": "query", "required": required, "schema": schema}
	}
	text := func(values ...string) map[string]any {
		s := map[string]any{"type": "string"}
		if len(values) > 0 {
			s["enum"] = values
		}
		return s
	}
	params := []any{}
	if route.Path == "/api/owner/soc/case" {
		params = append(params, parameter("id", true, map[string]any{"type": "string", "format": "uuid"}))
		if method == "get" {
			params = append(params, parameter("before_event", false, map[string]any{"type": "integer", "minimum": 1}))
		}
	} else {
		params = append(params, parameter("tier", false, text("L1", "L2", "L3")), parameter("status", false, text("active", "all", "open", "investigating", "contained", "resolved", "closed")), parameter("severity", false, text("info", "low", "medium", "high", "critical")), parameter("assignee", false, map[string]any{"type": "string", "maxLength": 120}), parameter("unassigned", false, text("1")), parameter("limit", false, map[string]any{"type": "integer", "minimum": 1, "maximum": 250, "default": 100}), parameter("offset", false, map[string]any{"type": "integer", "minimum": 0, "maximum": 100000, "default": 0}))
	}
	result := map[string]any{
		"operationId": operationID(method, route.Path), "summary": strings.ToUpper(method) + " " + route.Path,
		"description": "Owner-only operational case metadata. No verdict or enforcement authority. Queue telemetry covers the returned page. Missing storage returns 503, not a healthy or empty state. First-response SLA uses original creation severity and persisted response history.",
		"tags":        []string{"owner"}, "security": security("owner_session"), "parameters": params,
		"x-koschei-auth-tier": "owner_session", "x-koschei-inventory-auth": route.InventoryAuth, "x-koschei-route-source": route.InventorySource,
		"responses": map[string]any{
			"200": map[string]any{"description": "Operational queue, case and handoff history, or updated SOC state. Private response is not cacheable.", "content": map[string]any{"application/json": map[string]any{"schema": map[string]any{"type": "object", "required": []string{"ok"}, "properties": map[string]any{"ok": map[string]any{"const": true}}, "additionalProperties": true}}}},
			"400": response("Invalid filter, pagination, identifier or command.", "#/components/schemas/ErrorResponse"),
			"404": map[string]any{"description": "Owner authentication missing/invalid, or incident not found."},
			"405": map[string]any{"description": "Unsupported method."},
			"503": response("SOC storage, migration or query unavailable. No healthy state inferred.", "#/components/schemas/ErrorResponse"),
		},
	}
	if method == "post" {
		result["responses"].(map[string]any)["409"] = response("Expected revision is stale, or case is resolved/closed. Reload before retry.", "#/components/schemas/ErrorResponse")
		result["responses"].(map[string]any)["415"] = response("Content-Type must be application/json.", "#/components/schemas/ErrorResponse")
		result["requestBody"] = map[string]any{"required": true, "content": map[string]any{"application/json": map[string]any{"schema": map[string]any{
			"type": "object", "additionalProperties": false, "required": []string{"action", "summary", "expected_version"},
			"properties":  map[string]any{"action": text("assign", "escalate", "acknowledge"), "summary": map[string]any{"type": "string", "minLength": 1, "maxLength": 4000}, "expected_version": map[string]any{"type": "integer", "minimum": 0}, "assignee": map[string]any{"type": "string", "maxLength": 120}, "tier": text("L2", "L3")},
			"description": "16 KiB maximum body. assign accepts an assignee label (empty clears it); escalate requires a strictly higher tier; acknowledge accepts neither field. A nonblank reason and current revision are required. Actor cannot be supplied.",
		}}}}
	}
	return result
}
