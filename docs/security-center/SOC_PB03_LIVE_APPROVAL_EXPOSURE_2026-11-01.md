# SOC-PB-03 — Live EVM approval/spender exposure observation (2026-11-01)

## Operationally connected slice

The existing POST /api/scan/approval customer route **already** calls Ethereum-family
read-only JSON-RPC for an exact ERC-20 token + owner + spender:

1. Verify RPC eth_chainId is the requested EVM network.
2. Read the current ERC-20 allowance(owner,spender) by ABI-encoded eth_call.
3. Read current spender eth_getCode and parse the exact 23-byte EIP-7702
   delegation marker 0xef0100 + 20-byte delegation target.
4. Read ERC-1967 implementation, admin and beacon slots where available.
5. Independently validate these current observations and evaluate SOC-PB-03,
   returning result.threat_assessment in the **existing** approval scan v1 envelope.

This is a **wired, demand-driven detection surface**, not a scheduled EVM
mempool or full-chain attack detector. Its raw observation point is
"latest", not an exact shared pinned block. Delegation presence is not proof
of exploitation; standard proxy-slot absence does not prove immutability.

## Deterministic outcome contract

Schema: koschei.soc-approval-threat-observation.v1. The independent dimensions
are execution_state, evidence_state, finding_state, severity,
recommended_action, enforcement_state and report_integrity, plus reason_codes.

- If a valid current unlimited ERC-20 allowance is observed and spender EIP-7702
  delegation or ERC-1967 upgrade authority is observed:
  finding_state=unlimited_allowance_with_mutable_spender_authority,
  severity=high. This describes exposure, NOT an attack or intent verdict.
- Unlimited allowance without those observed control patterns: a separate
  medium-exposure observation, never a safe or allow decision.
- A finite nonzero allowance to a delegated or upgradeable spender: a
  medium-exposure observation for owner review.
- A zero allowance for the exact queried pair: a narrow zero observation,
  NOT a clean wallet bill of health. Historical/off-chain permits, Permit2,
  NFT operators and other token/spender pairs are not covered.
- If any authority probe fails or mismatches the allowance's network/spender,
  the finding is WITHHELD with evidence_state=incomplete or
  conflicting_or_unverified. Missing data never produces SAFE/ALLOW.
- Enforcement always reports not_connected. Report integrity always reports
  unsigned_read_only_observation. No synthetic Ed25519 verdict receipt is
  created. No SOC case is opened and no approval is revoked.

## Acceptance

Go unit corpus tests the high/medium/none and tampered/incomplete cases.
HTTP acceptance exercises the **same routed scan handler logic** with an
isolated HTTPS JSON-RPC server and verifies the actual eth_chainId,
eth_call, eth_getCode and eth_getStorageAt path, plus missing proxy evidence
and chain identity mismatch. The UI displays the bounded exposure signal and
its integrity/enforcement limits, escaping untrusted response strings.

These tests do NOT prove reliable detection of unknown attacks, a bounded
real-world alarm latency, a scheduled scanning service, HSM prevention,
customer owner consent, or 24x7 SOC operations. Those require separate
live operator acceptance with benign and adversarial transaction sampling,
false-positive measurement and a genuinely controlled pre-sign integration.

## Follow-up, by increasing authority

1. Add supported live event/head collectors with durable gap accounting and
   bounded source freshness; currently this endpoint runs on-demand only.
2. Correlate **state changes**, not just one current snapshot, for changing
   spender code and delegated authorization, retaining replayable evidence.
3. Validate an actual alarm-to-SOC case linkage with authorized owner action,
   stable identifiers, dedupe and read-back. Operator SLAs are not configured
   by this detector.
4. Only in an explicitly integrated signing path, verify state-bound
   permission and withhold signatures before broadcast. Do not claim the
   read-only approval scanner can stop a blockchain transaction.
