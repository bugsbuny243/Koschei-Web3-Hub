package handlers

// attachCanonicalArvisInvestigations is the shared no-I/O wiring point for the
// fourteen concrete ARVIS investigation answers. It projects only evidence
// already present in the canonical report, cannot collect network evidence, and
// cannot mutate deterministic rules, the signed verdict, or verdict authority.
func attachCanonicalArvisInvestigations(assembly *unifiedInvestigationAssembly) map[string]any {
	if assembly == nil {
		return map[string]any{}
	}
	if assembly.Report == nil {
		assembly.Report = map[string]any{}
	}
	investigations := buildArvisInvestigationProjection(assembly.Report)
	investigations = enrichArvisInvestigationProjection(investigations, assembly.Report)
	assembly.Report["arvis_investigations"] = investigations
	return investigations
}

// attachCanonicalArvisInvestigationsToReport supports direct canonical report
// surfaces that do not carry the full assembly after construction, such as the
// async worker result. It has the same projection-only semantics.
func attachCanonicalArvisInvestigationsToReport(report map[string]any) map[string]any {
	if report == nil {
		return map[string]any{}
	}
	investigations := buildArvisInvestigationProjection(report)
	investigations = enrichArvisInvestigationProjection(investigations, report)
	report["arvis_investigations"] = investigations
	return investigations
}
