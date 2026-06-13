# Production Data Audit Report

This audit removed user-facing demo, template, preview, and fabricated intelligence outputs so A.R.V.I.S and related flows either render retrieved production data, a partial report with per-module failures, or the exact emergency unavailability message:

> Real data unavailable. Analysis could not be completed.

## Removed or replaced fake/demo data sources

| Area | Removed source | Replacement behavior | Changed files |
| --- | --- | --- | --- |
| A.R.V.I.S unified command center | Browser-side orchestration of separate tools with UI-derived aggregate scoring and target-type placeholder prompts. | Frontend now calls `/api/v1/unified/analyze` and renders only returned module results, marking module errors/timeouts as partial failures. | `koschei/api/public/unified.html` |
| Unified backend endpoint | All modules ran for every target, producing skipped cards as if they were part of the report; total recommendations could imply normal monitoring when no production module completed. | Engine now selects applicable production modules by target type, uses the exact real-data-unavailable message on module failures, and the handler returns a 502 partial result without charging credits when no module completed. | `koschei/api/internal/services/unified_engine.go`, `koschei/api/internal/handlers/unified.go` |
| Smart Money B2B snapshot | Hardcoded whale cluster `solana_whales_alpha`, `$1,250,000` net flow, and `0.82` confidence. | Snapshot returns `503 real_data_unavailable` until production whale/CEX-flow sources are connected. | `koschei/api/internal/handlers/smart_money.go` |
| Smart Money stream | Static stream payload with unavailable wallet, zero flow, and B2B-ready flag. | Stream emits only an explicit real-data-unavailable event when live enrichment is unavailable. | `koschei/api/internal/handlers/smart_money.go` |
| Smart Money user watchlist | Hardcoded public account watchlist labels. | Authenticated endpoint now returns `503 real_data_unavailable` instead of shipping curated static accounts as intelligence. | `koschei/api/internal/handlers/portfolio_tracker.go` |
| Airdrop Checker UI | “Analiz şablonu yükle” preview button and no-backend preview JSON. | Preview path removed; only backend results or honest errors are shown. | `koschei/api/public/airdrop-checker.html` |
| Smart Money UI | “Analiz şablonu yükle” preview button and no-backend preview JSON. | Preview path removed; the UI calls the backend and displays real unavailability/errors. | `koschei/api/public/smart-money.html` |
| Cross-chain Risk UI | “Analiz şablonu yükle” preview button and fabricated template signals. | Preview path removed; only submitted backend results are rendered. | `koschei/api/public/cross-chain-risk.html` |
| Pay-per-tool UI | “price placeholders” labeling and per-tool “placeholder” suffix. | UI now describes configured prices without placeholder wording. | `koschei/api/public/pay-per-tool.html` |
| Agent API | Static wallet score `50`, safe metadata template with “Example asset”, and preliminary chain-health recommendation. | These routes now return `503 real_data_unavailable` until backed by production data. | `koschei/api/internal/handlers/intelligence_os.go` |
| Public module forms | Placeholder/example URLs and placeholder rendering in premium module fields. | Inputs no longer inject example values as placeholder content. Select fields still render real selectable values. | `koschei/api/public/js/koschei-premium-modules.js`, `koschei/api/public/*.html`, `koschei/api/public/css/koschei.css` |
| Artifact API response | `content_preview` response key could be mistaken for a demo preview. | Renamed to `content_excerpt`; prompt wording now asks for redacted configuration names instead of placeholders. | `koschei/api/internal/handlers/artifacts.go` |
| Documentation wording | SDK/API “Example(s)” labels in public docs. | Reworded to “Reference(s)” to avoid implying sample output in product flows. | `koschei/api/public/agent-api.html`, `koschei/api/public/docs.html`, `koschei/api/public/docs-sdk.html` |
| JARVIS module cards | Local variable named `preview` for truncated module display. | Renamed to `moduleExcerpt`; display remains a truncation of real returned module data. | `koschei/api/public/jarvis.html` |

## Remaining non-user-facing matches

The repository still contains `koschei/api/internal/mocks/*` and SQL variable names such as `placeholders` in `risk_scan.go`. These are test/support or query-construction artifacts, not user-facing demo intelligence or fabricated data sources.
