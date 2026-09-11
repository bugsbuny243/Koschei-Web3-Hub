# Web3 priority watch — 2026-09-10

Owner direction: focus current work on Koschei Web3. The product target remains
all blockchain networks and address types. Lang and Sentinel retain their
independent ownership and commercial direction.

This is a dated engineering assessment, not an automatic news feed or evidence
about a customer's address. The actions and acceptance cases below are proposed
work unless explicitly described as implemented. News does not create a SAFE or
AVOID verdict and does not authorize wallet interaction.

## What changed in this repository

Inspected main commit: `b972ffe5276890e604af069c2bc9575be39a6e45`.

| Change | Implemented scope | Remaining boundary |
| --- | --- | --- |
| [PR #1088](https://github.com/bugsbuny243/Koschei-Web3-Hub/pull/1088), merged September 10 | Explicit network/address resolution and coverage UI; network-scoped identity; no silent Solana fallback | Offline syntax checks. No additional live collector or all-network scan completion |
| [PR #1085](https://github.com/bugsbuny243/Koschei-Web3-Hub/pull/1085), merged September 10 | Read-only ARVIS historical-memory projection into the intelligence contract | Historical observations remain observed evidence; stored verdict metadata does not become a new decision or a verified signature |
| Dependency updates merged September 10 | Checkout, setup-node and Go crypto dependency updates | Their merge does not establish deployed product health |

These additions do not restore durable jobs, history/watchlists or application
workers whose persistence path is disabled. The established synchronous Solana
collection and ARVIS decision paths remain part of the product. See
[PROJECT_STATE.md](../PROJECT_STATE.md) for the existing open acceptance work.

## Current operational evidence

- On candidate `bf5346e194b6d8356b7bf302a5a20ca7d2cb3cde`,
  [API Required CI](https://github.com/bugsbuny243/Koschei-Web3-Hub/actions/runs/34428922352)
  passed. The [release gate](https://github.com/bugsbuny243/Koschei-Web3-Hub/actions/runs/34428922322)
  still reported `internal/http/network_target_routes.go` as unformatted.
  The follow-up change fixes that formatting without weakening the gate.
- [Public Product Smoke](https://github.com/bugsbuny243/Koschei-Web3-Hub/actions/runs/34428616996)
  observed HTTP 503 on `/api/public/cases?limit=100` across all 12 retries;
  the health endpoint returned 200. This is a recorded deployment observation,
  not a claim that the entire service is down. The production registry remains
  an acceptance blocker until a new deployed check proves recovery.

The subsequent read-only deployment inspection confirmed Railway deployment
`15c765c7-f13e-4a1c-8956-9be2f8399d86` runs the inspected main commit and has
deployment status SUCCESS. Its September 10, 02:19 UTC startup records confirm:

- Application PostgreSQL persistence is disabled. In
  [`main.go`](../koschei/api/main.go) the application handles are nil; the V2
  public-case loader requires the primary application DB and returns an error
  when it is nil. This explains the registry's unavailable state on this runtime.
  The new ClickHouse historical-memory projection does not supply this handle.
- `ENTITLEMENT_DATABASE_URL` is not set, and paid customer operations remain
  fail-closed. Existing `DATABASE_URL` is deliberately not a fallback. This is a
  separate commercial-access blocker, not a missing blockchain collector.

No variable values were retrieved or changed. Successful deployment therefore
does not establish that paid analysis or public-case discovery is operational.

## Verified external developments and product implications

The source statements and Koschei engineering inferences are separated below.
Publication dates are not assumed to be incident or activation dates.

| Published | Primary-source statement | Koschei implication / proposed action |
| --- | --- | --- |
| September 9, 2026 | Blockstream warned of impersonators following the Liquid incident, including fake support, recovery addresses and reimbursement/update lures. It states users are not required to move funds or disclose a recovery phrase. [Official warning](https://blog.blockstream.com/phishing-alert-do-not-act-on-unsolicited-liquid-or-blockstream-messages/) | Prioritize claim/URL and recovery-scam evidence in ARVIS. Preserve source URL, publication/observation dates and exact claimed scope. A general warning must never become a malicious-address label without address-specific evidence. No loss amount, exploit mechanism or recovery status is inferred from this warning. |
| September 3, 2026 | Solana describes the first phase of rent reduction and `WithdrawExcessLamports`, which can move surplus SOL without closing a token account or changing its token balance. [Official developer update](https://solana.com/news/how-to-reclaim-excess-sol-after-rent-reduction) | Review transaction interpretation against account owner, signer/authority, destination, current rent floor and pre/post balances. A lamport decrease alone cannot establish a token drain. The whole transaction still needs review; the instruction name alone cannot establish safety. |
| August 24, 2026 | Ethereum's upcoming Glamsterdam gas repricing can break assumptions in some contracts, wallets and infrastructure; the foundation provides devnet testing guidance. [Official repricing notice](https://blog.ethereum.org/2026/08/24/glamsterdam-repricing-testing) | Make EVM collection/simulation reports identify chain ID, block hash, fork context and estimation source. Exercise hardcoded-gas and changed-state-cost cases on the relevant test network before claiming compatibility. Do not present a planned upgrade as already activated on mainnet. |
| August 17, 2026 | Solana analyzes a proposed v1 transaction layout with a larger envelope and no Address Lookup Tables. [Official v1 analysis](https://solana.com/news/transaction-v1-and-the-alt-trade-off) | Track decoder/version compatibility. The current services transaction fetch explicitly requests `maxSupportedTransactionVersion: 0`; changing this number alone would not implement v1. Require version-specific parsing and fixtures before expanding support. |

## Ordered work and acceptance

1. **P0 — Restore paid customer access.** Provision/connect the dedicated
   entitlement ledger through the existing split-plane contract. Acceptance:
   real plan lookup, atomic quota reservation/refund and paid-request lifecycle
   succeed against the configured ledger. Do not substitute the application
   database or a fabricated plan. Configuration and real-ledger acceptance remain
   outstanding; this change does not configure a service or initiate billing.
2. **P0 — Recover verifiable public case access.** Implement the separate
   publication-persistence path required by the stateless boundary. Preserve
   primary-store revocation visibility, entitlement and evidence
   checks. Acceptance: the deployed registry is operational with verifiable cases
   or a truthfully complete, healthy empty result; the existing smoke gate passes.
   Returning fabricated cases or treating a 503 as healthy is not a fix.
3. **P0 — Bind security advisories to evidence.** Add a versioned advisory input
   behind ARVIS's chain-independent boundary, including original URL, publisher,
   publication and observation times, affected scope, and review state. Acceptance:
   customers see source and freshness; unrelated addresses are not accused;
   retracted/stale advisories are visible; missing evidence stays UNKNOWN.
   This input and its frontend are not implemented by this document.
4. **P1 — Add real cross-network evidence.** Extend the network resolver with an
   EVM evidence adapter and subsequently other network families. Acceptance:
   verify configured versus observed chain identity, block/finality context,
   source and age; the same address on different networks remains distinct;
   collection failures and unsupported networks are visible in the UI.
5. **P1 — Protect Solana interpretation across upgrades.** Exercise authorized
   excess-rent withdrawal, wrong authority/destination, token-balance changes,
   legacy/v0 transactions, and unsupported/malformed versions. Acceptance:
   supported cases retain existing behavior; unsupported data cannot create a
   clean security result; transaction-size constants are version-specific only
   when the relevant format is actually implemented and verified.
6. **P1 — Test EVM fork compatibility.** Use the announced test environment for
   changed gas semantics. Keep testnet evidence separate from mainnet evidence.
   Recheck the upstream activation status before implementation or release.

This ordering addresses current customer access and evidence quality while
continuing the all-network product scope. It does not add a live news crawler,
new collectors, paid execution, or production configuration changes.
