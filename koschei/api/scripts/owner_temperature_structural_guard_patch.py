from pathlib import Path


def replace_once(text: str, old: str, new: str, label: str) -> str:
    if old not in text:
        raise SystemExit(f"missing replacement target: {label}")
    return text.replace(old, new, 1)

# 1) Anthropic Owner: make it structurally impossible to serialize temperature.
path = Path("internal/router/anthropic_owner.go")
text = path.read_text()
text = replace_once(
    text,
    '\tMaxTokens   int                     `json:"max_tokens"`\n\tTemperature float64                 `json:"temperature,omitempty"`',
    '\tMaxTokens int                     `json:"max_tokens"`',
    "remove anthropic temperature payload field",
)
text = replace_once(
    text,
    '''\tif req.Temperature < 0 || req.Temperature > 1 {
\t\treq.Temperature = 0.2
\t}
\tif req.Temperature == 0 {
\t\treq.Temperature = 0.2
\t}
''',
    '''\t// HARD RULE: never send `temperature` to the Anthropic API. Claude
\t// Sonnet 5 rejects that legacy parameter with HTTP 400. Keep temperature
\t// absent from anthropicOwnerPayload itself so future call-site changes
\t// cannot accidentally serialize it back into owner requests.
''',
    "replace temperature normalization with hard rule",
)
text = replace_once(
    text,
    '''\tpayload := anthropicOwnerPayload{
\t\tModel:       model,
\t\tSystem:      strings.TrimSpace(req.System),
\t\tMessages:    []anthropicOwnerMessage{{Role: "user", Content: req.Prompt}},
\t\tMaxTokens:   req.MaxTokens,
\t\tTemperature: req.Temperature,
\t}
''',
    '''\tpayload := anthropicOwnerPayload{
\t\tModel:     model,
\t\tSystem:    strings.TrimSpace(req.System),
\t\tMessages:  []anthropicOwnerMessage{{Role: "user", Content: req.Prompt}},
\t\tMaxTokens: req.MaxTokens,
\t}
''',
    "remove temperature payload assignment",
)
path.write_text(text)

# 2) Safe Check: preserve the already-merged structural alignment contract.
path = Path("internal/handlers/arvis_preflight.go")
text = path.read_text()
text = replace_once(
    text,
    '\tresp := evaluateARVISPreflight(req)\n\tresp = h.alignARVISPreflightWithStructuralBaseline(r.Context(), req, resp)',
    '''\tresp := evaluateARVISPreflight(req)
\t// HARD RULE: a cached, fresh, verified structural floor may raise Safe
\t// Check, but must never lower a stronger local phishing/signature verdict.
\tresp = h.alignARVISPreflightWithStructuralBaseline(r.Context(), req, resp)''',
    "safe check hard rule comment",
)
path.write_text(text)

# 3) Structural memory: document the separate-freshness invariant.
path = Path("internal/services/security_radar_structural.go")
text = path.read_text()
text = replace_once(
    text,
    '// StructuralBaseline exposes the strongest fresh, verified structural floor\n// for quick read paths such as Safe Check. A quick heuristic answer must never\n// contradict stronger holder/authority evidence Koschei already verified.',
    '''// StructuralBaseline exposes the strongest fresh, verified structural floor
// for quick read paths such as Safe Check. A quick heuristic answer must never
// contradict stronger holder/authority evidence Koschei already verified.
// HARD RULE: holder and authority freshness remain independent; do not replace
// their timestamps with a single aggregate observed_at value.''',
    "structural baseline hard rule comment",
)
path.write_text(text)
