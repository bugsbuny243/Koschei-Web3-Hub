# Universal ARVIS — Multi-Network + Web4/Web5/Web6 Research Intake

**Date:** 2026-09-18  
**Scope:** Koschei Web3 / ARVIS  
**Classification:** research + implementation plan; repository code is not deployment proof

## 1. Truth boundary

ARVIS must become **target-first and evidence-first**, not token-first.

The production foundation remains Web3. The labels **Web4, Web5 and Web6 are working architecture profiles in Koschei**, not accepted W3C/IETF generations of the Web. The implementation must therefore bind those labels to concrete protocols, standards, cryptographic evidence and explicit maturity states.

Rules:

- A registered network is not automatically a live network.
- A syntactically valid address is not proof that an account exists.
- A probe-ready collector is not a full investigation adapter.
- A verified identity does not imply unlimited authority.
- Agent capability does not imply authorization.
- A submitted transaction/message does not prove final execution.
- Missing evidence remains unknown.
- Chain identity must be proven before chain-specific evidence is accepted.
- Cross-chain actor identity must never be inferred from address similarity alone.

## 2. Universal target taxonomy

ARVIS should route evidence by **target kind before applying any security rule**.

Initial universal kinds:

- address / wallet
- token / asset
- contract
- program / module
- transaction / message / receipt
- block / checkpoint / ledger version
- validator / sequencer / consensus participant
- bridge / cross-chain route
- governance object / proposal / authority
- identity / DID
- verifiable credential
- agent
- protocol / tool server
- artifact / signed output
- project

A token-specific rule must never be applied to a wallet, bridge, agent, credential or arbitrary contract merely because they share an address-like identifier.

## 3. Chain-family adapter model

The universal core should normalize evidence into the existing ARVIS intelligence contract:

- subject
- entity
- evidence
- relationship
- behavior
- hypothesis
- attack path
- decision

Chain-specific collectors stay behind adapters.

### Solana / account-program family

**Repository state:** live production foundation.

Evidence anchors:

- network identity
- slot
- transaction signature
- program ID
- parsed instruction
- account ownership
- token/SOL deltas
- mint-specific token accounts
- durable actor memory

Existing deterministic Solana verdict semantics remain unchanged.

### EVM family

**Repository state:** probe-ready for configured networks.

Registered probe networks in this implementation step:

- Ethereum
- Base
- Arbitrum
- Optimism
- Polygon
- BNB Smart Chain
- Avalanche C-Chain

Every EVM probe must verify `eth_chainId` before accepting state. Contract bytecode, EIP-7702 delegation and ERC-1967 authority are observations, not automatic malicious/safe verdicts.

Next evidence expansion:

- transaction + receipt binding
- logs/events
- internal call traces where provider evidence supports them
- ERC-20/721/1155 state
- approval / permit / spender surfaces
- implementation/admin/beacon authority
- bridge deposit/fill/finalization evidence
- governance/timelock control
- deployer/factory relationships
- deterministic behavior rules before grade promotion

Official references:

- Ethereum JSON-RPC: https://ethereum.org/developers/docs/apis/json-rpc/
- Polygon PoS network/reference docs: https://docs.polygon.technology/
- BNB Smart Chain network docs: https://docs.bnbchain.org/
- Avalanche C-Chain / EVM docs: https://build.avax.network/

### Bitcoin / UTXO family

**Repository state:** probe-ready.

Evidence anchors:

- Bitcoin genesis/network identity
- block hash
- transaction ID
- inputs / outputs
- scriptPubKey
- UTXO state

Future behavior analysis must distinguish transaction-graph observations from ownership claims. Address reuse, peel-like topology or consolidation patterns are not sufficient by themselves to assert common control.

Official reference:

- Bitcoin Core RPC: https://bitcoincore.org/en/doc/

### Move family — Sui + Aptos

**Repository state:** planned.

Sui adapter evidence targets:

- objects and object ownership/version
- packages/modules
- transaction digest
- checkpoint
- events
- capabilities and shared objects

Aptos adapter evidence targets:

- accounts/resources/modules
- transaction version/hash
- events
- object model
- validator/ledger state

Official references:

- Sui docs: https://docs.sui.io/
- Aptos developer docs: https://aptos.dev/

### Cosmos SDK / CometBFT family

**Repository state:** planned.

Evidence targets:

- chain ID
- block height/hash
- transaction hash
- ABCI/module state
- events
- validator set
- governance
- IBC packet/channel/client evidence

Cross-zone or IBC relationships remain OBSERVED until source and destination evidence are independently anchored.

Official reference:

- CometBFT RPC: https://docs.cometbft.com/

### Substrate / Polkadot family

**Repository state:** planned.

Evidence targets:

- genesis/chain identity
- block hash
- runtime version
- runtime metadata
- storage
- extrinsics
- events
- governance / proxy / multisig / XCM-related evidence

Runtime metadata is version-bound. A decoder for one runtime version must not silently interpret another runtime.

Official reference:

- Polkadot developer docs: https://docs.polkadot.com/

### TON family

**Repository state:** planned.

Evidence targets:

- workchain
- account state
- contract code/data
- inbound/outbound messages
- transaction
- shard/block anchor

Message intent and final account-state effect must be represented separately.

Official reference:

- TON docs: https://docs.ton.org/

### NEAR account/Wasm family

**Repository state:** planned.

Evidence targets:

- chain ID
- account
- Wasm code
- access keys
- transaction
- receipts
- block/hash
- execution outcome

Receipt execution/finality must be proven separately from submission.

Official reference:

- NEAR docs: https://docs.near.org/

## 4. Web4 working profile — agents, tools, delegation and provenance

**Status in Universal ARVIS:** research / observe-first.

Concrete primitives rather than a “Web4 standard”:

### Model Context Protocol (MCP)

The 2026-07-28 MCP specification introduced a stateless protocol core, header-based routing, extensions and authorization hardening.

ARVIS evidence targets:

- MCP server identity / origin
- protocol version
- advertised tools/resources/prompts
- authorization context
- tool invocation request/response
- declared vs observed side effects
- task/session provenance
- signed artifact/result hashes where available
- policy boundary and denied operations

Official reference:

- https://blog.modelcontextprotocol.io/posts/2026-07-28/

### Agent2Agent (A2A)

A2A 1.0.0 defines interoperability between independent agents, including capability discovery, interaction modalities, collaborative tasks and artifact exchange.

ARVIS evidence targets:

- agent identity and endpoint
- declared capability
- task lifecycle
- request/response provenance
- artifact hashes
- authorization/delegation context
- cross-agent trust boundary

Official reference:

- https://a2a-protocol.org/dev/specification/

Security rule: **capability != authorization != execution != effect**. These four axes must never be collapsed into one boolean.

## 5. Web5 working profile — portable identity and verifiable data

**Status in Universal ARVIS:** research / observe-first.

Concrete standards:

### W3C Decentralized Identifiers

DID Core 1.0 is a W3C Recommendation. DID 1.1 and DID Resolution continued through the W3C standards process in 2026.

Evidence targets:

- DID method
- DID document resolution
- controller
- verification method
- service endpoints
- resolution metadata
- key rotation/deactivation
- method-specific trust assumptions

References:

- https://www.w3.org/TR/did-core/
- https://www.w3.org/TR/did-1.1/
- https://www.w3.org/TR/did-resolution/

### W3C Verifiable Credentials 2.0

The VC 2.0 family became W3C Recommendations on 2025-05-15.

Evidence targets:

- issuer
- subject
- credential type
- securing mechanism/proof
- verification method
- validity window
- status/revocation state
- disclosed claims
- evidence/provenance

References:

- https://www.w3.org/TR/vc-data-model-2.0/
- https://www.w3.org/news/2025/the-verifiable-credentials-2-0-family-of-specifications-is-now-a-w3c-recommendation/

Security rule: credential verification proves only the claims and controller relationships actually covered by the credential/proof. It never creates unrestricted authorization.

## 6. Web6 working profile — autonomous security bounds and crypto agility

**Status in Universal ARVIS:** research / observe-first.

There is no universal “Web6 standard”. Koschei uses Web6 as a working profile for verifiable autonomous-compute boundaries, crypto agility and post-quantum transition evidence.

NIST finalized the first PQC standards in 2024:

- FIPS 203 — ML-KEM
- FIPS 204 — ML-DSA
- FIPS 205 — SLH-DSA

ARVIS evidence targets:

- algorithm inventory
- key/certificate identity
- artifact/signature provenance
- signing and verification result
- key lifecycle
- classical/PQ transition state
- unsupported/deprecated algorithm state
- migration boundary
- crypto-provider/version evidence

Official reference:

- https://www.nist.gov/news-events/news/2024/08/nist-releases-first-3-finalized-post-quantum-encryption-standards

Security rule: the appearance of a PQ algorithm name is not proof of PQ readiness. Provider implementation, key provenance, algorithm parameters, signed artifact and migration state must be independently evidenced.

## 7. Cross-chain intelligence model

Cross-chain intelligence must never mean “one address = one actor everywhere”.

A verified cross-chain relationship needs chain-specific evidence on both sides and a bridge/routing event that binds them.

Minimum bridge evidence schema:

- source network
- source transaction
- source block/finality
- source sender/contract
- bridge protocol + version
- message/deposit identifier
- asset + amount
- destination network
- destination transaction/fill/finalization
- destination recipient/contract
- evidence status per side
- independent source references

Only when both sides are verified may ARVIS create a VERIFIED cross-chain relationship. Partial paths remain OBSERVED or UNVERIFIED.

## 8. Implementation sequence

### P0 — Universal investigation core

- universal target-kind taxonomy
- chain-family adapter registry
- evidence/trust-anchor declaration
- explicit maturity status
- same-address cross-network separation
- no verdict promotion for planned/research adapters

### P1 — EVM expansion

- extend the existing chain-identity-verified probe to Polygon, BNB Smart Chain and Avalanche C-Chain
- keep deployment/configuration truth separate from code support
- preserve observed-only status until deterministic EVM rules exist

### P2 — Full EVM evidence adapter

- transaction + receipt
- logs/events
- bytecode + storage authority
- token/NFT state
- approvals/permits
- deployer/factory
- bridge/governance evidence
- attack-path and behavior projection

### P3 — Additional chain-family observers

Implement adapters in order of evidence maturity, not market hype:

- Move
- Cosmos/CometBFT
- Substrate
- TON
- NEAR

Each adapter must prove chain identity before analysis.

### P4 — Cross-chain graph

- bridge event adapters
- source/destination transaction binding
- canonical asset/route references
- evidence-backed cross-network relationships
- no address-similarity identity inference

### P5 — Web4/Web5/Web6 evidence adapters

- MCP/A2A agent/task/artifact provenance
- DID/VC identity and credential verification
- crypto-agility/PQ evidence profile

These start observe-only and cannot alter deterministic production grades until separate promotion acceptance is executed.

## 9. Acceptance rules

A Universal ARVIS adapter is not “complete” because a schema or file exists.

Promotion requires:

1. chain/protocol identity verification;
2. canonical target normalization;
3. bounded read-only collector;
4. source + time + block/transaction/proof provenance;
5. VERIFIED / OBSERVED / INFERRED / UNVERIFIED semantics;
6. missing evidence remains unknown;
7. fixture and live acceptance tests;
8. telemetry and operator-visible status;
9. no regression to existing Solana evidence semantics;
10. explicit decision on whether evidence is allowed to affect a deterministic verdict.

## 10. Current implementation delta — 2026-09-18

This branch begins P0 and P1:

- adds a Universal ARVIS adapter/profile registry;
- adds universal target kinds beyond coins/tokens;
- keeps Move/Cosmos/Substrate/TON/NEAR planned;
- keeps Web4/Web5/Web6 research-only;
- expands the existing EVM chain-ID-verified probe family to Polygon, BNB Smart Chain and Avalanche C-Chain;
- preserves Solana as the current strong production evidence core;
- preserves Bitcoin and EVM probe evidence as evidence-bound, not automatic safety verdicts.
