# Token-2022 Security Scanner

Koschei extends the existing Go token-scan handler without changing the existing request contract.

## Endpoint

```http
POST /api/token/scan
Authorization: Bearer CUSTOMER_SESSION_TOKEN
Content-Type: application/json

{
  "mint": "SOLANA_TOKEN_MINT",
  "network": "solana-mainnet"
}
```

Existing classic SPL Token responses remain compatible. The response now includes additive fields:

- `token_program`
- `token_2022`
- `extensions`
- `extension_risk_penalty`
- `transfer_behavior`
- `visibility_limitations`
- `compatibility_warnings`
- `final_policy`

## HTML surface

```text
/token-2022-scanner
```

The page uses plain HTML, CSS and browser JavaScript. It uses the customer's existing Koschei session and never requests a private key or seed phrase.

## Extension rules

The first Go rule set evaluates:

| Extension | Default severity | Why it matters |
| --- | --- | --- |
| PermanentDelegate | Critical | Delegate may transfer or burn holder balances |
| TransferHook | High | Custom program runs on every transfer |
| TransferFeeConfig | Medium to critical | Protocol-level transfer fee changes received amount |
| MintCloseAuthority | High | Mint account can be closed by an authority |
| DefaultAccountState | Medium to high | New accounts may start frozen |
| NonTransferable | Medium | Token cannot follow normal transfer flows |
| ConfidentialTransferMint | Medium | Public amount visibility can be incomplete |
| ConfidentialTransferFeeConfig | High | Fee behavior can be confidential |
| ConfidentialMintBurn | High | Mint or burn activity can be confidential |
| PausableConfig | High | Authority may pause all transfers |
| ScaledUiAmountConfig | Medium | Displayed balances can differ from raw balances |
| InterestBearingConfig | Low | UI amount includes an interest calculation |
| MetadataPointer | Low | Metadata is resolved through a pointer |
| TokenMetadata / group extensions | Informational | Metadata and grouping behavior |

Unknown extensions are not silently treated as safe. They generate a compatibility warning until a dedicated rule is added.

## Policy output

`final_policy` is one of:

- `allow`
- `warn`
- `block`

Critical extensions produce `block`. High or medium extensions produce `warn`. Confidential features produce at least `warn` because ordinary public analysis may have visibility limitations.

The policy is a security signal, not a claim that every use of an extension is malicious. Token-2022 extensions are legitimate protocol features, but they can materially change token behavior and integration assumptions.

## Data source

The scanner reads the mint account through Solana RPC with `jsonParsed` encoding. The account owner and parser identify classic SPL Token versus Token-2022. Active extensions and their state are read from the parsed mint information.
