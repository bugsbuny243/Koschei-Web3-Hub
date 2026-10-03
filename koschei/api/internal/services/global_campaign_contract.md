# Global Campaign v1

`koschei.global-campaign.v1` is an evidence-referenced campaign projection over existing Koschei Web3 security artifacts.

It does not create a second actor graph and does not replace Global Radar relations, bridge links, Campaign Genome, Campaign Tempo, Incident Corpus, Behavioral Signatures, Attack Path, or signed ARVIS verdicts.

The contract is intentionally non-authoritative:

- no verdict authority;
- no grade authority;
- no containment authority;
- no real-world identity claim;
- no same-operator claim;
- no wrongdoing claim.

A stable `campaign_ref` identifies the operational campaign across revisions. Each revision receives a canonical `evidence_hash_sha256`; material evidence changes therefore change the hash without changing the campaign reference.

The initial contract is storage-agnostic. Persistence, lifecycle transition validation, convergent materialization, cross-chain temporal correlation, Sentinel Fabric opinion exchange, incident linkage, and authorized response proof are follow-on slices.
