# Koschei Don-2N Shield API

Don-2N Shield is the B2B preflight layer for wallets, DEXs, bots, launchpads, and security products.

It lets an integration ask Koschei before a user signs or submits a risky action.

## Authentication

Use a Koschei API key through the `X-API-Key` header.

Do not place server API keys inside public browser apps. For wallets and web apps, proxy the request through your backend or issue scoped client tokens.

## Preflight endpoint

`POST /api/v1/shield/preflight`

Request fields:

- `target_mint`
- `target`
- `address`
- `transaction`
- `wallet`
- `network`

Response fields:

- `action`: `allow`, `allow_with_monitoring`, `warn`, or `block`
- `grade`: A-F
- `risk_index`: numeric score
- `risk_level`: low, medium, high, or critical
- `reason`: human-readable evidence reason
- `signed`: whether the verdict is signed
- `signature`: signed verdict hash

## Transaction endpoint

`POST /api/v1/shield/transaction`

The transaction endpoint uses the same engine as preflight and accepts `transaction`, `target`, `target_mint`, `address`, and `wallet` fields.

## Browser SDK

The public SDK is available at `/sdk/koschei-shield.js`.

Example flow:

```js
const shield = new KoscheiShield({
  baseURL: 'https://tradepigloball.co',
  getToken: async () => await getScopedTokenFromYourBackend()
})

const verdict = await shield.preflight({
  target_mint: mintAddress,
  wallet: walletAddress,
  network: 'solana-mainnet'
})

if (verdict.action === 'block') {
  throw new Error('Koschei Shield blocked this transaction')
}
```

## Safety line

- AI never changes final grade.
- No evidence means evidence gap, not fake certainty.
- API keys stay in env or server-side storage.
- SBX-1 stream evidence can enrich the verdict, but final scoring remains deterministic and rule-based.
