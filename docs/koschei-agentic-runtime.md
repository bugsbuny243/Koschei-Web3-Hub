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

## Phase 5.3 stabilization notes
- Phase 5.3 is planning/contract only.
- Phase 6 will consume `file_plan` and `proposed_tool_calls` for artifact generation.
- Proposed tool calls are not executed in Phase 5.3.
- Human approval is mandatory for high-risk, security, government, bank, and smart-glasses workflows.
- The async runtime worker flow is required (`processRuntimeProject`).
- Old sync runtime generation flow must not be used.

## Phase 6 — Artifact & Code Package Generation
- Runtime Contract 5.3 now feeds Koschei Artifact Builder.
- file_plan entries are transformed into generated_files rows.
- proposed_tool_calls remain proposed-only and are not executed.
- Generated package is downloadable as zip.
- No shell execution, no repo write, no deploy in this phase.

## Phase 6.1 — Artifact Generation Stabilizer
- Artifact generation is hardened for safer parsing, preview routing, zip assembly, and credit behavior.
- Phase 6.1 final fix adds transaction failure safety so failed artifacts cannot remain in `processing`.
- Protected ZIP download on web now uses authenticated fetch (Bearer token), while mobile keeps protected-link placeholder behavior.
- Artifact-specific env vars and strict JSON schema prompt are now part of Phase 6.1 hardening.
- Generated files are stored in database tables (`generated_artifacts`, `generated_files`) only.
- Generated code is not executed.
- No shell execution and no repo writes are performed by artifact generation.
- Proposed tool calls are still not executed.
