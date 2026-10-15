package handlers

// unifiedToken2022ExtensionProjection keeps parsed extension observations
// separate from any final grade. An owner observation alone does not establish
// whether Token-2022 extensions were completely resolved.
func unifiedToken2022ExtensionProjection(source map[string]any) map[string]any {
	out := map[string]any{
		"status": "unverified", "evidence_complete": false,
		"extension_count": 0, "extension_names": []string{},
	}
	owner := unifiedMintOwnerEvidenceProjection(source)
	if owner["token_2022"] == false {
		out["status"] = "not_applicable"
		return out
	}
	if owner["token_2022"] != true {
		return out
	}
	raw, ok := source["token_2022_extension_evidence"].(map[string]any)
	if !ok {
		return out
	}
	status, _ := raw["status"].(string)
	if status == "token_2022_extensions_unresolved" {
		out["status"] = status
		out["limitation_code"] = "TOKEN_2022_EXTENSIONS_UNRESOLVED"
		return out
	}
	if status != "resolved" && status != "resolved_empty" {
		return out
	}
	complete, ok := raw["evidence_complete"].(bool)
	if !ok || !complete {
		out["status"] = "token_2022_extensions_unresolved"
		out["limitation_code"] = "TOKEN_2022_EXTENSIONS_UNRESOLVED"
		return out
	}
	count, ok := raw["extension_count"].(int)
	if !ok || count < 0 {
		return out
	}
	names, ok := raw["extension_names"].([]string)
	if !ok || count != len(names) || (status == "resolved" && count == 0) || (status == "resolved_empty" && count != 0) {
		return out
	}
	out["status"] = status
	out["evidence_complete"] = true
	out["extension_count"] = count
	out["extension_names"] = append([]string{}, names...)
	return out
}
