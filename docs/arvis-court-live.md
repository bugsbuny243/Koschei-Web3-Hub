# ARVIS Tribunal — production rollout

ARVIS Tribunal is a read-only explanation and review layer above the signed deterministic verdict. It never changes the grade, verdict, signature, rule evidence or threat-pathway classification.

## Seats

| Stage | Provider/model environment |
|---|---|
| Lead prosecutor | `TOGETHER_MODEL_PROSECUTOR_LEAD` |
| Independent evidence prosecutor | `TOGETHER_MODEL_PROSECUTOR_EVIDENCE` |
| First-instance member 1 | `TOGETHER_MODEL_TRIBUNAL_QWEN` |
| First-instance member 2 | `TOGETHER_MODEL_TRIBUNAL_GLM` |
| Senior member 1 | `OPENAI_MODEL_TRIBUNAL` |
| Senior member 2 | `ANTHROPIC_OWNER_MODEL` |

Default lower-court model identifiers are:

```text
moonshotai/Kimi-K2.6
MiniMaxAI/MiniMax-M3
Qwen/Qwen3-235B-A22B-2507
zai-org/GLM-5.2
```

The deployed provider catalog must accept the configured identifiers. Do not put API keys in the repository.

## Required Railway variables

```text
KOSCHEI_COURT_ENABLED=true
KOSCHEI_COURT_PROSECUTORS_ENABLED=true
KOSCHEI_COURT_PANEL_ENABLED=true
KOSCHEI_COURT_SENIOR_ENABLED=true

TOGETHER_API_KEY=<secret>
TOGETHER_MODEL_PROSECUTOR_LEAD=moonshotai/Kimi-K2.6
TOGETHER_MODEL_PROSECUTOR_EVIDENCE=MiniMaxAI/MiniMax-M3
TOGETHER_MODEL_TRIBUNAL_QWEN=Qwen/Qwen3-235B-A22B-2507
TOGETHER_MODEL_TRIBUNAL_GLM=zai-org/GLM-5.2

OPENAI_API_KEY=<secret>
OPENAI_BASE_URL=https://api.openai.com/v1
OPENAI_MODEL_TRIBUNAL=<configured model id>

ANTHROPIC_API_KEY=<secret>
ANTHROPIC_OWNER_MODEL=<configured model id>
```

Optional controls:

```text
KOSCHEI_COURT_MODEL_TIMEOUT=60s
KOSCHEI_COURT_HTTP_TIMEOUT=75s
KOSCHEI_COURT_PROVIDER_RETRIES=1
KOSCHEI_COURT_QUOTA_PRO_DAILY=20
KOSCHEI_COURT_QUOTA_ENTERPRISE_DAILY=100
KOSCHEI_OWNER_COURT_AUTO_ENABLED=false
KOSCHEI_OWNER_COURT_EXTENDED=false
```

The owner UI explicitly requests the full court for each manual owner scan. `KOSCHEI_OWNER_COURT_AUTO_ENABLED` only controls non-UI owner callers that omit the request field.

## Runtime behavior

### Free

No court route and zero model calls.

### Basic

No court route and zero court model calls. The deterministic scan and signed verdict remain available under the existing Basic contract.

### Pro

`POST /api/v1/radar/court`

- Kimi and MiniMax produce independent opinions.
- Qwen and GLM run only when a deterministic rule changes the grade, the prosecutors disagree or one prosecutor is unavailable.
- The daily court budget applies in addition to the normal scan quota.

### Enterprise

Uses the same endpoint with `{"extended":true}` when a full senior review is requested.

- OpenAI and Anthropic run in parallel when configured.
- A D/F deterministic grade, a first-instance panel or explicit extended review can invoke the senior panel.
- One frontier provider may fail without discarding the other provider or the lower-court file. The report is marked `partial`.

### Owner

The Owner ARVIS scan posts:

```json
{
  "target": "<solana target>",
  "network": "solana-mainnet",
  "live_evidence": true,
  "court": true,
  "extended_court": true
}
```

The resulting docket is rendered below the deterministic verdict and Threat Anticipation report.

## Evidence contract

Every provider receives a bounded, immutable packet containing the signed verdict plus available deterministic evidence:

- Threat Anticipation pathways and limitations
- behavior-rule results
- actor rule verdict and actor evidence
- owner-resolved holder intelligence
- holder-cluster evidence
- market snapshot
- ARVIS modules and evidence rows
- deterministic graph data

The provider prompt forbids:

- numeric risk scores or numeric rug probability,
- real-person identity or bad-intent claims,
- invented evidence, signatures or rule identifiers,
- converting `INFERRED` or `UNVERIFIED` data into fact,
- changing grade, verdict or signature.

## Safe deployment order

1. Deploy the code with `KOSCHEI_COURT_ENABLED=false`.
2. Confirm API CI, database connectivity, ordinary Radar and owner scans.
3. Add model environment variables without exposing secrets.
4. Enable `KOSCHEI_COURT_ENABLED=true`.
5. Run one owner case and confirm both prosecutors.
6. Trigger a grade-changing or disagreement case and confirm Qwen/GLM.
7. Confirm OpenAI/Anthropic only on Enterprise/owner extended cases.
8. Watch provider latency, quota use and partial/error states before increasing daily limits.

## Failure policy

- Provider errors never change or remove the signed deterministic verdict.
- A missing lower-court provider produces a partial file when another opinion is available.
- Total lower-court failure produces `status=error`.
- Senior-provider failure keeps the lower court and produces `status=partial`.
- Missing or disabled configuration returns `court_unavailable`; it is never presented as a safe finding.
