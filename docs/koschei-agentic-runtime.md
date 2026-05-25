# Koschei Agentic Runtime (Phase 5.3)

Koschei uses agent contracts to transform a single blueprint response into a staged runtime pipeline with structured outputs.

## Why agent contracts
- Stable machine-readable stages for intake, planning, review, and delivery.
- Better guardrails and review routing before artifact generation.
- Phase 6 readiness without executing tool calls or file writes.

## Model roles
- Qwen/Coder: code and file planning.
- Llama: chat and explanation.
- DeepSeek: reasoning and review.
- Flux/Veo/Kling/Kokoro/Whisper: media workers.

## OpenAI-style inspirations (architecture only)
- Structured outputs
- Function-calling style tool proposals
- Agents-as-tools
- Guardrails
- Human-in-the-loop
- Routing/triage
- Parallel judge

## Phase 5.3 constraints
- OpenAI SDK is **not** added.
- Together remains the primary provider.
- Tool calls are proposed only (not executed).
- Real artifact generation starts in Phase 6.
