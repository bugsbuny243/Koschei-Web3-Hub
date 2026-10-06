# Web3–Web8 Daily Intelligence — 2026-10-06

## New signal: Ethereum Glamsterdam is now observed on Sepolia

On 2026-10-06 at epoch 353024 / slot 11296768, Ethereum's Glamsterdam upgrade activated on Sepolia. This is a real public-testnet state transition, not a mainnet activation. Hoodi and mainnet remain unscheduled.

Glamsterdam combines Amsterdam (execution) and Gloas (consensus), including protocol-level proposer-builder separation (EIP-7732), block-level access lists (EIP-7928), and revised execution/state-growth gas accounting.

A late Prysm v7.2.1 release corrected Sepolia's default Gloas gas-limit configuration to 200M and adjusted builder-auth configuration semantics shortly before activation. This is useful evidence that “fork supported by client” and “client configured correctly for activated capability” must be separate states.

### Koschei Web3 opportunity
Extend Protocol Lifecycle Radar with:
`ANNOUNCED -> CLIENT_SUPPORTED -> CLIENT_CONFIGURED -> TESTNET_ACTIVATED -> OBSERVED_CONSENSUS -> STABILITY_WINDOW -> MAINNET_SCHEDULED -> MAINNET_ACTIVATED`.

Add `CLIENT_CONFIGURATION_CONTINUITY`: track client/version/config combinations against the canonical fork schedule and observed chain state.

### Threat / competitor gap
News feeds commonly collapse testnet and mainnet state. Upgrade trackers commonly track release compatibility but not configuration drift. ARVIS can combine network observation, client compatibility, activation state and evidence strength.

### Product action
Expose protocol capability state with network, activation epoch/height, minimum compatible client, configuration-sensitive fields, observed finality and evidence source.

## Standards-status control
No accepted W3C/IETF “Web4”, “Web5”, “Web6”, “Web7” or “Web8” generation standard was identified today. These remain labels/visions unless tied to concrete standards artifacts. Agent-protocol Internet-Drafts must remain classified as work in progress, not standards.

## Sources
- Ethereum Foundation, “Glamsterdam Testnet Announcement”, 2026-09-28; activation occurred 2026-10-06.
- ethereum/pm Glamsterdam deployments: Sepolia epoch 353024, slot 11296768.
- Prysm v7.2.1 release reporting/configuration change, 2026-10-05.
