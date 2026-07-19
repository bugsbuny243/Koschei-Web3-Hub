# Koschei Solana Defense Agent Runtime — Phase 1

Phase 1 connects the future Solana Program Security Lab to the existing Unified Investigation without replacing or weakening the current system.

## Constitutional boundary

The defense-agent runtime is shadow-only in version 1.

- It cannot change the signed deterministic grade, verdict, signature or ruleset.
- It cannot send a mainnet transaction.
- It cannot modify source code.
- It cannot call a compiler, fuzzer, sandbox, model or external artifact source yet.
- Missing program artifacts remain `evidence_pending`; they are never interpreted as safety.
- Runtime or persistence failure never removes the existing token, actor, LP, market or threat report.

The existing Koschei deterministic engine remains the only verdict authority.

## Environment

```text
KOSCHEI_DEFENSE_AGENT_RUNTIME_ENABLED=false
```

Keep the flag disabled for the first deployment. After migration 065 is applied, enable it for owner acceptance tests:

```text
KOSCHEI_DEFENSE_AGENT_RUNTIME_ENABLED=true
```

Version 1 always runs in shadow mode. There is deliberately no active-execution switch.

## Initial agents

### Program Archaeologist

Reads only the existing Unified Investigation file and resolves program IDs already present in fields such as LP program, loader, source context and structural evidence. It reports missing source, IDL and bytecode artifacts.

### Static Analyzer

Registered but blocked until a verified source, IDL or sBPF bytecode artifact is attached. No vulnerability claim is produced from missing artifacts.

### Reproduction Agent

Registered but blocked until a program-security finding is independently verified and reachable. Mainnet execution is prohibited.

## Initial tools

- `resolve_program_surface`
- `extract_instruction_graph`
- `run_static_detectors`
- `prepare_reproduction_plan`

Every tool contract is read-only and has `can_change_verdict=false`. Sandbox-required tools remain unavailable in Phase 1.

## Unified Investigation contract

Every report receives:

```json
{
  "defense_agent_runtime": {
    "schema_version": "koschei-defense-agent-runtime-v1",
    "execution_mode": "disabled|shadow",
    "verdict_authority": false,
    "can_execute_mainnet": false,
    "can_modify_source": false,
    "agents": [],
    "tool_invocations": [],
    "input_hash": "sha256:...",
    "report_hash": "sha256:..."
  }
}
```

The evidence policy also states that the runtime cannot change the verdict, execute on mainnet or modify source.

## Immutable persistence

Migration `065_defense_agent_runtime.sql` creates:

- `defense_agent_runs`
- `defense_tool_invocations`

Both tables are append-only. Update and delete operations are rejected by database triggers. Each run and tool envelope includes deterministic identifiers and SHA-256 input/output hashes.

## Acceptance criteria

1. Existing Unified Investigation results remain unchanged when the feature flag is disabled.
2. When enabled, a bounded shadow report is attached even when no program artifact exists.
3. Program IDs already in the existing report are resolved without network access.
4. Static analysis and reproduction remain blocked rather than inventing results.
5. Database failure is reported as persistence status and does not fail the customer scan.
6. All new agents and tools have no verdict authority.
7. PostgreSQL migrations, Go tests, vet and Linux build pass.

## Next phase

Phase 2 adds the artifact intake and knowledge layer:

- verified source repository and commit binding,
- Anchor IDL ingestion,
- deployed sBPF binary hash and source/binary comparison,
- versioned Solana/Anchor knowledge documents,
- Neon pgvector retrieval,
- temporal program/account/CPI graph,
- sandbox job queue without mainnet signing capability.
