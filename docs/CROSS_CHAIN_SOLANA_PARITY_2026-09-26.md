# Cross-chain Solana parity program — 2026-09-26

## Goal

Raise every supported production network to the same Koschei evidence standard as the Solana live core without pretending that identical chain mechanics exist.

Parity means equivalent security outcomes, not identical collectors:

- deterministic target classification;
- live transaction/state evidence;
- authority/control-surface evidence where the chain has that concept;
- relationship/flow graph evidence;
- monitoring and retained history;
- multi-provider disagreement handling;
- evidence-bound ARVIS decisions;
- signed verdict authority only after chain-specific rules and acceptance tests are complete.

## Current baseline

- Solana: live core with the deepest token, wallet, transaction, program, liquidity and ARVIS paths.
- EVM: live address, bytecode, EIP-7702, ERC-1967, ERC-20 allowance, transaction/receipt, block/log ingest and node/finality telemetry paths; deterministic EVM verdict authority is still disconnected.
- Bitcoin: address activity, node/PoW telemetry, block/transaction-identity ingest and evidence graph primitives exist; customer transaction lookup is the first parity gap selected for closure.
- Move (Sui/Aptos): identity-only and remains fail-closed.

## Ordered parity work

1. Transaction parity
   - Bitcoin customer tx lookup with Esplora evidence — delivered.
   - EVM selector + standard Transfer/Approval layout evidence — delivered (hint-only selectors; no ABI-name overclaim).
   - EVM calldata/log semantic decoding beyond standard layouts — next.
   - Bitcoin input/output/prevout/script/value flow projection — delivered.

2. Asset and control parity
   - EVM ERC-20/721/1155 metadata, holders/concentration, implementation/admin/delegation graph.
   - Bitcoin UTXO ownership-free flow graph now has transaction-local prevout/output evidence; cross-transaction spend graph and spend-condition semantics remain next.

3. Liquidity / market structure
   - EVM DEX pool/vault/liquidity evidence with protocol-specific adapters.
   - Bitcoin exchange/bridge relations only when independently evidenced.

4. Monitoring
   - Per-chain durable cursors, reorg handling, provider disagreement and retained alert history.

5. Decision authority
   - Chain-specific deterministic rules and golden corpora.
   - Ed25519 signed verdicts only after evidence acceptance gates pass.

## Non-negotiable boundary

No network is promoted to Solana-level product status because an endpoint exists. Missing collectors remain unknown, provider disagreement is explicit, and chain-specific concepts are marked not-applicable rather than imitated.
