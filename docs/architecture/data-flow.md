# ARVIS Data Flow

## 1. Source observation

ARVIS receives observations from Solana launch and liquidity surfaces.

Current production focus:

- Pump-style launch activity
- Raydium-style liquidity activity
- Solana RPC transaction and account enrichment

## 2. Normalization

Raw provider-specific fields are converted into a stable observation object:

```text
source
module_id
event_type
signature
target
target_type
network
observed_at
metadata
```

The open-source event normalizer demonstrates this boundary.

## 3. Queue and idempotency

Eligible observations enter the processing queue. The same stream event and evidence arm must not create duplicate verdicts.

Operational states include:

- processing
- completed
- insufficient evidence
- retryable failure
- exhausted failure

## 4. Evidence arms

Independent arms inspect categories such as:

- launch behavior
- liquidity context
- mint and freeze authority
- holder concentration
- wallet and funding relations
- transaction behavior
- claim surface risk

Each arm should remain unsigned when its required evidence is unavailable.

## 5. Final verdict

The final engine combines verified evidence into a customer-facing object:

```text
grade
risk_index
risk_level
verdict
recommendation
evidence
rule_version
signed
signature
```

## 6. Delivery

The same final contract is delivered through:

- live radar
- customer reports
- session-authenticated analysis
- API-key partner routes
- future webhooks and SDK integrations

## 7. Billing boundary

A successful evidence-backed analysis may consume one output. Evidence collection failure should not be charged. API-key routes reserve credits and refund qualifying processing failures.
