# Unified Attack-Path Security Model

## Purpose

Koschei Web3 remains a Web3 Security Validation & Risk Intelligence platform. This model extends the existing chain-neutral ARVIS intelligence contract so a Web3-relevant attack path can cross trust boundaries without forcing unrelated security engines into this repository.

The core question is not only "is this wallet or contract risky?". The model must be able to represent:

`Subject -> Identity/Authority -> Capability -> Action -> Trust Boundary -> State Change -> Consequence -> Evidence`

A subject can be a wallet/address, contract, treasury, oracle, AI agent, credential-backed identity, device, machine, asset, validation case, or another adapter-supplied actor. Declaring a subject kind does not claim that Koschei Web3 currently has a live collector for that domain.

## Security domains

The v1 envelope recognizes these adapter domains:

- `blockchain`: wallets, contracts/programs, bridges, treasuries, oracles, governance, transactions and state changes.
- `ai_agent`: agent identity, tool use, delegated authority, task boundaries and spend-capability evidence that can reach digital assets or Web3 control planes.
- `identity`: portable credentials, impersonation evidence, grants, revocations and authorization state relevant to digital-asset access.
- `information_model`: provenance for documents, knowledge inputs, model-produced evidence and decision inputs that influence Web3 actions.
- `virtual_world`: avatar/asset identity or environment state when it controls or represents on-chain ownership/value.
- `machine_economy`: device identity, machine wallets, M2M payments and commands with digital-economic or on-chain consequences.
- `post_quantum`: cryptographic inventory, signer/key migration and compatibility evidence for Web3 identities and protocols.
- `protocol`: new protocol adapters and cross-system control surfaces that do not fit a chain family directly.

Unknown domains fail closed as `unknown`.

## Repository boundary

This repository does **not** implement a generic LLM security platform, DID stack, robotics security suite, digital-twin engine, firmware scanner or post-quantum cryptography runtime.

Those systems are external evidence producers or adapter targets only when they affect Web3 authority, assets, protocol state or attack paths.

Koschei Sentinel is not an active integration target in this repository. Any future external intelligence producer must use an explicit, versioned evidence contract rather than importing or copying external internals.

Koschei Lang remains a separate language/runtime project and is not a production dependency of Koschei Web3. Web3 must not reproduce Lang internals.

## Evidence invariants

The unified contract is evidence-first and fail-closed:

1. A capability cannot become `verified`, `observed` or `inferred` without concrete evidence references.
2. Evidence status and capability lifecycle are separate. A capability can be **verified as revoked**; revocation is not the same as missing evidence.
3. A trust-boundary transition requires a subject, recognized source and destination domains, a transition mechanism and evidence references.
4. A path cannot retain a non-`unverified` status if any path step lacks evidence references.
5. A consequence without evidence remains unverified. Inferred simulation impact stays `inferred`; it is not silently upgraded to observed loss.
6. Confidence is bounded metadata about the evidence statement. It is not a universal risk probability.
7. Solana and other case-sensitive native identifiers remain byte/text exact; the model must not lowercase native subject IDs.
8. The existing ARVIS intelligence decision remains authoritative. The unified envelope must not re-grade the base decision.

## Completed projection binding checks

Adapters call `FinalizeUnifiedSecurityInvestigation` after assembling the complete
envelope. Builders start with `binding_status=unchecked`; a consistent projection
reports `binding_status=consistent`. This checks graph consistency, not producer
authentication, current permission to act, or a safe customer verdict.

Every referenced subject, capability, action, transition and evidence row must
resolve without ambiguous IDs. Evidence must belong to a participating subject or
the explicitly referenced resource. Raw resource addresses must also match a
participant's chain/network context, so the same address on another network cannot
supply evidence accidentally. Declared transaction hashes must have matching
evidence, and conflicting transaction hashes are rejected. Claims cannot be stronger
than their supporting evidence or referenced capabilities/actions/transitions.
Attack-path entries, step order and linked actors/targets must agree.

A broken binding sets `binding_status=unverified` and returns machine-readable
`binding_issues` with the affected object ID and reason code. All additive
capabilities, actions, transitions, paths and consequences become `unverified`
with zero confidence. The original base evidence, decision and caller-owned
claims are preserved. Consistent observed/inferred/unavailable evidence is never
upgraded, and a verified revoked or prospective capability retains that lifecycle.

Transaction Guard and Defense Validation finalize their projections before
returning the additive envelope. Domain adapters still own semantic interpretation
and producer authentication; a consistent graph alone does not prove causation.

## Authenticated signed evidence ingress

The unified model can consume the repository's existing `securityevidence.Event` format through an authenticated ingress adapter. This is a trust boundary, not a generic claim importer.

The ingress requires all of the following before a signed event can become unified intelligence evidence:

- the event digest must verify;
- Ed25519 producer authentication must verify against a trusted public key supplied out of band by the consuming control;
- the declared producer must match the configured expected producer;
- the signed event subject must match the configured expected subject binding;
- the unified domain and subject kind must be recognized;
- the event must contain at least one finding.

The producer public key is never accepted from the event itself. Subject `chain` and `type` namespaces are canonicalized, while the native subject ID remains exact and case-sensitive. A subject substitution therefore fails even when it differs only by ID casing.

A signed event does not automatically create capabilities, actions, trust-boundary transitions, attack paths or customer decisions. The generic ingress projects only authenticated subject/evidence statements. Domain-specific adapters must perform any later semantic mapping and remain responsible for proving those semantics.

Evidence state is preserved conservatively:

- `VERIFIED` becomes unified `verified` only when the underlying `securityevidence.Event` is itself valid and its finding satisfies the event contract's verified-evidence digest requirement;
- `OBSERVED` remains `observed`;
- `UNAVAILABLE` and `NOT_APPLICABLE` are not upgraded and project as `unverified` evidence state.

Every projected evidence row preserves the event SHA-256, producer, finding identity/state, finding evidence digest when present, source digests, observation window and authenticated producer subject namespace as provenance attributes.

For authenticated blockchain evidence, recognized namespaces such as `solana` and `eip155:*` can be mapped to the existing Solana/EVM chain-family vocabulary. Non-blockchain evidence remains `chain_family=unknown`; the system does not pretend an agent, identity or protocol namespace is a blockchain.

## Defense Validation projection

Koschei Defense Validation is the first existing signed-evidence producer wired through the generic trust boundary and then interpreted by a domain-specific mapper.

The mapper only projects validation cases that already passed the Defense Validation execution-recomputation and independent-observer checks. For each such case it:

- re-authenticates the signed independent observation through the unified signed-evidence ingress;
- binds the validation-case subject to the exact case reference, chain namespace, collector identity and server-supplied collector public key;
- links the signed observation to the deterministic Defense Validation report hash, scenario contract hash, ruleset, control, technique, execution mode, evidence hashes, timing result and outcome;
- emits an evidence-linked validation action and behavior finding;
- leaves the unified customer decision `unverified/investigate` instead of copying or re-grading the Defense Validation report verdict.

A verified attack-path gap is emitted only for an authenticated attack case whose deterministic outcome is `missed` or `caught_late`. `caught_in_time`, `clean`, `false_positive` and `incomplete` are not represented as attack-path blind spots.

Even a verified validation gap carries explicit boundaries: the evidence applies to the exact isolated fork/sandbox scenario and is **not** a claim that the production protocol was exploited, compromised or lost assets. The consequence describes a validated defense-control gap, not observed production damage.

Cases without an independently authenticated observation do not produce unified subjects, actions, evidence-backed behaviors or attack paths. Missing observation evidence therefore remains missing instead of being inferred from the scenario or execution proof alone.

## Example cross-domain path

A future adapter chain may produce:

`Modified knowledge input -> AI agent -> spend capability -> treasury API -> signer -> bridge transaction -> destination contract -> asset/state consequence`

Koschei Web3 should represent this as one evidence-linked path while preserving the provenance of every transition. It must not claim the upstream document, agent or device was compromised unless the corresponding adapter supplied authenticated evidence for that claim and a domain-specific mapper established the semantics.

## Current production truth

ARVIS has production evidence paths centered on its existing Web3/Solana collectors and chain-neutral projections. Transaction Guard has an additive projection into the unified model for evidence-backed pre-signing authority/capability and attack-path semantics.

Defense Validation now also projects its existing recomputed execution evidence and independently signed observations into the same unified evidence model. This does not change Defense Validation's deterministic verdict authority, execution-containment rules or its explicit non-mainnet/non-production-mutation boundary.

The signed-evidence ingress establishes a cryptographic adapter boundary, but it does not make AI-agent, identity, information/model, virtual-world, machine-economy or post-quantum collectors production-live. Those domain values remain contract vocabulary until a real evidence producer, trust configuration and domain-specific adapter are deployed and verified.

No UI, package or commercial capability should advertise those domains as live solely because this data contract or signed ingress exists.

## Evolution

The intended evolution remains:

`Address -> Entity -> Identity/Authority -> Capability -> Transaction/Action -> Behavior -> Attack Path -> Consequence -> Evidence-backed Decision`

New protocols should enter through adapters that map native evidence into this contract. Core intelligence must not be hard-wired to one chain or one external security system.
