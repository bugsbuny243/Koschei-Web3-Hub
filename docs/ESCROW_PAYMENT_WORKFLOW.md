# Escrow Payment Workflow

## Policy

Escrow.com fees are treated as TradePi Globall internal operating costs. They are included in internal quote calculation and are not displayed as separate charges to the customer or supplier.

## Quote model

Internal calculation model:

- supplier landed cost
- + escrow fee paid by TradePi Globall
- + bank/wire/operation costs paid by TradePi Globall
- + TradePi Globall markup/profit
- = final customer quote

Public customer quote remains one final price only.

## Escrow transaction creation

- Fee payer defaults to `tradepi_globall` in internal records.
- Buyer-pays-fee and seller-pays-fee options must not be exposed on public pages.
- If Escrow API requires fee payer party details, the decision must remain in server/admin logic only.
- Supplier payment tracking remains separate (`T/T 30% advance`, `T/T 70% balance before delivery`).
- Escrow fee must not reduce supplier payment amounts.
