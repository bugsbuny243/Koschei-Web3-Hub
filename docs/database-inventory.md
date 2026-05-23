# Koschei Database Inventory (No-Delete Safety)

This document is the safety contract for database usage. Legacy tables are retained but **must not** be used by new runtime code.

## Active Koschei Core Tables (allowed for new runtime code)

- `schema_migrations`
- `plans`
- `payment_requests`
- `credits_ledger`
- `generation_jobs`
- `runtime_projects`
- `runtime_tasks`
- `runtime_logs`
- `model_route_logs`

## Owner/Internal Tables

- `schema_migrations` (migration tracking)
- `plans` (catalog of allowed plans)
- `payment_requests` (manual payment workflow)
- `credits_ledger` (single source of truth for runtime credits)

## Legacy Duplicate / Old Tables (read-only historical, no new writes)

- `credit_events`
- `credit_ledger`
- `app_users`
- `users`
- `user_projects`
- `ai_generations`
- `subscriptions`

## Legacy TradePi/B2B Tables (do not use for Koschei runtime)

- `products`
- `product_media`
- `quote_requests`
- `suppliers`
- `supplier_leads`
- `supplier_messages`
- `supplier_ddp_quotes`
- `customer_quotes`
- `customer_final_quotes`
- `escrow_transactions`
- `tradepi_commissions`
- `documents`
- `market_research_jobs`
- `market_research_reports`
- `market_research_sources`
- `supplier_discovery_jobs`
- `supplier_outreach_messages`
- `supplier_outreach_events`
- `supplier_payment_milestones`

## Code Usage Rules

New code is allowed to use only active Koschei core tables unless explicitly approved.

Critical protections:

- Credits must use `credits_ledger` only.
- Do **not** use `credit_events`.
- Do **not** use `credit_ledger`.
- Do **not** use `users`/`app_users` for runtime auth yet.
- Do **not** use `user_projects` for runtime projects.
- Plan IDs are limited to: `free`, `starter`, `pro`, `studio`.
- Paid activation plan IDs are limited to: `starter`, `pro`, `studio`.
- `builder` is deprecated and must not be accepted.
