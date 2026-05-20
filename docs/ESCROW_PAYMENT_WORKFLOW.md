# Escrow Payment Workflow (TradePi Globall Machinery)

- Escrow.com is used only **after** customer quote approval.
- No public checkout is enabled.
- No public prices are displayed for machinery pages.
- Customer pays Escrow.com via an admin-created transaction.
- Supplier payments are tracked manually as T/T 30% advance and 70% balance before delivery.
- Webhook notifications are never trusted alone; API fetch verification is required before final status update.
- Sandbox first, production only after real account and key verification.
- Escrow fees must be included in customer quote calculation or explicitly assigned to buyer/seller.

## Required environment variables

```env
ESCROW_ENV=sandbox
ESCROW_API_BASE_URL=https://api.escrow-sandbox.com/2017-09-01
ESCROW_EMAIL=
ESCROW_API_KEY=
ESCROW_WEBHOOK_TOKEN=
ESCROW_DEFAULT_SELLER_EMAIL=
ESCROW_DEFAULT_CURRENCY=usd
```

Production later:

```env
ESCROW_ENV=production
ESCROW_API_BASE_URL=https://api.escrow.com/2017-09-01
```

## Railway setup

Set all variables in Railway service environment. Keep `ESCROW_API_KEY` server-only and never expose it to client bundles.
