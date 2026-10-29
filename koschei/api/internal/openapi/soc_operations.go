package openapi

import "strings"

func socOperation(route Route, method string) map[string]any {
	if route.Path == "/api/owner/soc/readiness" {
		// A read-only bounded storage probe is not a case-management API or SLA.
		// Do not inherit SOC case pagination, mutation or response-time claims.
		readinessResponse := func(ok bool) map[string]any {
			return map[string]any{
				"type": "object", "additionalProperties": true,
				"required": []string{"ok", "readiness"},
				"properties": map[string]any{
					"ok":        map[string]any{"const": ok},
					"readiness": map[string]any{"type": "object", "additionalProperties": true},
				},
			}
		}
		body := func(description string, ok bool) map[string]any {
			return map[string]any{
				"description": description,
				"content":     map[string]any{"application/json": map[string]any{"schema": readinessResponse(ok)}},
			}
		}
		return map[string]any{
			"operationId":              operationID(method, route.Path),
			"summary":                  strings.ToUpper(method) + " " + route.Path,
			"description":              "Owner-only read-only PostgreSQL SOC storage readiness probe. It checks bounded case/action read access and linked samples without exposing case data. No staffing, response, incident containment, SLA, or end-to-end security acceptance is proven.",
			"tags":                     []string{"owner"},
			"security":                 security("owner_session"),
			"parameters":               []any{},
			"x-koschei-auth-tier":      "owner_session",
			"x-koschei-inventory-auth": route.InventoryAuth,
			"x-koschei-route-source":   route.InventorySource,
			"responses": map[string]any{
				"200": body("Read path queried; readiness.state may still be not accepted. No production readiness is implied.", true),
				"404": map[string]any{"description": "Owner authentication absent or invalid."},
				"405": map[string]any{"description": "Only GET is permitted."},
				"503": body("Storage unavailable or required probe failed; always unavailable_not_accepted.", false),
			},
		}
	}
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
