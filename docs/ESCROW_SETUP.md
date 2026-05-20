# Escrow.com Setup (Foundation Layer)

This project uses a **server-side only** Escrow.com foundation for TradePi Globall Machinery.

## Railway environment variables

Configure the following variables in Railway:

- `ESCROW_ENV`
- `ESCROW_API_BASE_URL`
- `ESCROW_EMAIL`
- `ESCROW_API_KEY`
- `ESCROW_DEFAULT_SELLER_EMAIL`
- `ESCROW_DEFAULT_CURRENCY`
- `ESCROW_FEE_PAYER`
- `ESCROW_WEBHOOK_TOKEN`

## Deployment and safety rules

- Start in **sandbox** (`ESCROW_ENV=sandbox`) first.
- Do **not** create a public checkout.
- Do **not** create a real customer payment button.
- Do **not** expose escrow keys.
- Do **not** show public prices.
- Do **not** show supplier costs or TradePi margin publicly.
- Do **not** create fake products.
- Do **not** auto-pay supplier.
- Keep all Escrow API calls on the server side.

## Escrow fee policy

Escrow.com fees are paid by **TradePi Globall** as internal operating cost.

- Customer does not pay escrow fee separately.
- Supplier does not pay escrow fee.

## Supplier payment operations (outside escrow automation)

Supplier payments remain manual T/T tracking:

- 30% advance payment
- 70% before delivery
