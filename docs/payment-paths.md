# Koschei payment paths

## Active payment path: Paddle

Paddle is the active package payment provider for Starter, Professional, and Enterprise.

1. An authenticated customer calls `POST /api/paddle/checkout` or `POST /api/v1/paddle/checkout` with a selected package.
2. The backend maps the package to a Paddle price ID from environment variables and creates a Paddle transaction. The frontend never sends or controls the price.
3. A pending `orders` row is recorded when Paddle returns a transaction/session identifier.
4. Signed Paddle webhooks are verified with `PADDLE_WEBHOOK_SECRET` before any payload is processed.
5. Successful transaction/subscription events write real `orders` records and upsert the customer's active `entitlements` row.
6. Customer premium access is read from active, non-expired `entitlements`; no active entitlement means no premium analysis.

## Legacy payment path: Shopier / payment_requests

`payment_requests` is retained for legacy Shopier/manual review flows and owner panel visibility.

1. Customers can still submit legacy payment requests where enabled.
2. Owner approval activates an entitlement using the existing manual review path.
3. Shopier webhook code is preserved and still records approved legacy requests.
4. Legacy records are not deleted or migrated destructively.

Paddle purchases should use `orders` + `entitlements`; `payment_requests` remains a legacy/manual operational table.
