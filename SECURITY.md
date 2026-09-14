# Security Policy

Koschei Web3 handles security-sensitive Web3 evidence and integrations. Security reports must avoid exposing secrets, private customer data, or actionable exploit details in public issues.

## Reporting a vulnerability

Do **not** open a public GitHub issue containing exploit steps, credentials, private keys, seed phrases, session tokens, provider secrets, customer-sensitive evidence, or other sensitive material.

Use GitHub's private security-reporting channel for this repository when it is available. If private reporting is not available, contact the repository owner through a private channel and provide only the minimum information needed to establish impact and reproduce the issue safely.

A useful report should include:

- affected component and revision/commit;
- security impact and affected trust boundary;
- minimal reproduction conditions;
- whether the issue affects confidentiality, integrity, availability, authorization, evidence provenance, entitlement, signing, or decision correctness;
- any safe logs or redacted evidence needed to reproduce the behavior.

## Secret handling

Never submit or commit:

- wallet private keys or seed phrases;
- passwords or session/bearer tokens;
- RPC/provider/API secrets;
- bot or webhook secrets;
- database credentials;
- customer-sensitive case/evidence data.

If a real secret is exposed, treat it as compromised and rotate/revoke it outside the repository. Removing a secret from the current tree is not sufficient if it exists in Git history.

## Product security invariants

Security fixes must preserve the repository contract in `AGENTS.md`, including:

- missing evidence remains `UNKNOWN` rather than becoming `SAFE`;
- entitlement and authorization paths fail closed;
- chain adapters cannot mint customer decision authority;
- Sentinel output is evidence interpretation, not Web3/Lang authority;
- Fabric integrations are versioned and begin observe/shadow-first;
- signing/effect controls cannot be bypassed by external protocol adapters.

## Validation

Security-related changes should run the relevant exact-head CI and acceptance gates. Passing a unit test alone is not a production-readiness claim. See `PROJECT_STATE.md` for open blockers and current acceptance status.
