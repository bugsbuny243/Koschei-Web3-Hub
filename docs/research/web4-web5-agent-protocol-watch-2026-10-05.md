# Agent protocol standards watch — 2026-10-05

Three meaningful standards developments were found.

## DIV (draft-janbjer-div-00)
Published 2026-10-04. Defines transport-independent signed action approvals and offline verification. It distinguishes signed approval from the later action result, and treats single-use handling as separate state.

Koschei Lang signal: keep IntentApprovalEvidence, ActionPermission, ActionResult, and ReplayState independent.

Source: https://datatracker.ietf.org/doc/draft-janbjer-div/

## AACP (draft-levi-agent-certification-00)
Published 2026-10-04. Defines short-lived cryptographic credentials showing that a particular agent configuration passed a defined evaluation profile. The credential is evidence of demonstrated capability, not permission to access a resource.

Koschei Lang signal: CapabilityEvidence must remain distinct from AuthorityGrant, and evidence should be configuration-bound and time-bounded.

Source: https://datatracker.ietf.org/doc/draft-levi-agent-certification/

## AIC X.509 extension (draft-wei-aic-identity-cert-02)
Revision -02 published 2026-10-02. Binds an agent cryptographic identity to a natural-person principal while deliberately leaving capability and policy semantics outside the certificate.

Koschei Lang signal: IdentityBinding must remain distinct from DelegatedAuthority. X.509, DID/VC, SPIFFE and on-chain identity mechanisms should remain interchangeable evidence carriers.

Source: https://datatracker.ietf.org/doc/draft-wei-aic-identity-cert/

## Architectural convergence
IDENTITY_BINDING != CAPABILITY_EVIDENCE != ACTION_PERMISSION

SIGNED_INTENT != ACTION_PERMISSION != ACTION_RESULT
