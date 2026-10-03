# Koschei Global Campaign v1

Date: 2026-10-02

## Purpose

`koschei.global-campaign.v1` is the canonical campaign-level correlation contract for Koschei Web3 Global Radar.

It projects references to existing evidence and intelligence artifacts into one stable campaign identity without creating a second actor/entity graph and without changing deterministic ARVIS authority.

## Identity model

A campaign has two distinct identities:

- `campaign_ref`: stable operational identity across material revisions;
- `evidence_hash_sha256`: immutable canonical digest of one exact campaign revision.

New evidence may change the revision, state, entity set, reference set, or evidence hash while preserving the same campaign reference.

## Inputs by reference

The contract may reference existing:

- Global Radar observations;
- Global Radar relation edges;
- verified bridge links;
- Campaign Genome snapshots;
- Campaign Tempo fingerprints;
- Behavioral Signature findings;
- Security Incident Corpus records;
- signed verdict references;
- Attack Path findings.

The contract does not duplicate the underlying evidence payloads.

## Authority boundary

Global Campaign is correlation context only.

It has no authority to:

- create or change an ARVIS grade;
- mint or replace a signed deterministic verdict;
- release, block, contain, or execute an action;
- merge different wallet addresses into one real-world identity;
- assert common control;
- assert wrongdoing.

All authority flags are forced false by canonicalization and rejected if a persisted/materialized representation violates the boundary.

## Validation boundary

A materialized campaign must have:

- canonical schema version;
- non-empty stable campaign reference;
- positive revision;
- valid lifecycle state;
- at least one evidence anchor or evidence reference;
- canonical evidence digest;
- no decision, containment, identity, common-control, or wrongdoing authority.

## Current slice

This first slice is storage-agnostic and contains only the canonical contract, deterministic hashing, normalization, validation, documentation, and tests.

Persistence, lifecycle transition rules, convergent materialization, cross-chain temporal correlation, Sentinel Fabric opinion exchange, incident linkage, response authorization, and effect verification remain separate follow-on slices.
