# Koschei Unified Professional Integration v1

Status: implementation branch contract

## Product model

Koschei is one commercial security ecosystem with one Professional entitlement.

- **Koschei Web3 Hub** is the customer security center and production Web3 security surface.
- **ARVIS** is the deterministic evidence, investigation and signed-verdict engine inside Web3 Hub.
- **Koschei Sentinel** is the evidence-grounded cybersecurity intelligence/model layer. It integrates through Koschei Fabric in observe-first mode and cannot replace or mutate an ARVIS signed verdict.
- **Koschei Lang** is the independent capability-secure programming language, compiler/runtime, Matrix/policy layer and developer toolchain. It is commercially included in Professional while retaining its own repository, semantics and release gates.

Codebases remain federated. "Unified" means one product contract, entitlement, workspace and integration plane; it does not mean copying Lang or Sentinel internals into Web3.

## One official token

There is one official Koschei ecosystem token: **KOSC**.

- Network: Solana mainnet
- Mint: `7X9V77axASFAV8hKqqn2EfyAz4Qz3tceN8iikfukLqy1`
- Scope: Web3 Hub + Sentinel + Lang
- No separate Sentinel token
- No separate Lang token
- Holdings alone never grant Professional access
- KOSC settlement activates the same server-side Professional entitlement only after verified finalized payment

## ARVIS -> Telegram customer delivery

The existing customer Telegram pairing/consent store is reused. A second bot or second identity store is not introduced.

Flow:

1. Customer pairs the configured Koschei Telegram bot through the existing pairing flow.
2. A customer-triggered ARVIS scan completes.
3. The HTTP response remains the canonical result.
4. A customer-specific Telegram delivery is queued for every successful scan, including evidence-pending results.
5. The delivery worker re-checks active consent before provider I/O.
6. Telegram failure never changes the deterministic ARVIS result.

System high/critical security alerts remain a separate operator channel and are not used as a substitute for customer result delivery.

## Sentinel Web3 adapter

The Web3 adapter projects only a signed, live-evidence ARVIS result into Sentinel's `sentinel.case.v1` contract.

Sentinel is skipped when:

- ARVIS has no live evidence;
- the final verdict is unsigned;
- the verdict signature is missing/invalid;
- there are no concrete ARVIS evidence statements to project.

Sentinel responses are accepted only when the returned `case_id` and `verdict_signature` exactly match the submitted case. The response is attached as `sentinel_observation` with `mode=observe` and `authority=commentary_only_arvis_verdict_final`.

Required Web3 runtime configuration when the adapter is deliberately enabled:

```env
KOSCHEI_SENTINEL_OBSERVE_ENABLED=1
KOSCHEI_SENTINEL_BASE_URL=https://<sentinel-service>
KOSCHEI_SENTINEL_API_TOKEN=<runtime-secret>
```

The API token is a runtime secret and must never be committed.

## Lang Professional inclusion

Professional is the commercial umbrella for Koschei Lang licensed developer tooling. Lang remains authoritative for:

- `.ks` language semantics;
- compiler/runtime behavior;
- capability/authority rules;
- Matrix policy/proof internals;
- editor/toolchain behavior;
- Lang release and compatibility gates.

Web3 may expose entitlement/status/download or adapter surfaces, but it must not become the source of truth for Lang semantics.

## Remaining deployment gates

1. Deploy the Sentinel service behind its authenticated `/v1/opinions` boundary.
2. Set the matching Web3 Sentinel URL/token runtime secrets.
3. Verify an evidence-backed ARVIS scan receives observe-only Sentinel commentary without changing the ARVIS signature.
4. Verify a paired Professional customer receives the same ARVIS scan result through Telegram.
5. Define the licensed Lang artifact/distribution channel used by Professional customers; entitlement inclusion does not fabricate an artifact that has not been published.
6. Run repository CI and production acceptance before merging/enabling runtime switches.
