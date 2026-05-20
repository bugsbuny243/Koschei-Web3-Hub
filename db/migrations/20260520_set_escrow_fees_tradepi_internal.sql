alter table if exists escrow_transactions
  add column if not exists escrow_fee_payer text default 'tradepi_globall',
  add column if not exists escrow_fee_amount numeric,
  add column if not exists escrow_fee_currency text default 'USD',
  add column if not exists escrow_fee_paid_by_tradepi boolean default true,
  add column if not exists escrow_fee_public_note text,
  add column if not exists internal_fee_note text;

alter table if exists customer_quotes
  add column if not exists supplier_landed_cost numeric,
  add column if not exists escrow_fee_internal numeric,
  add column if not exists bank_transfer_fee_internal numeric,
  add column if not exists operation_cost_internal numeric,
  add column if not exists internal_total_cost numeric,
  add column if not exists markup_amount numeric,
  add column if not exists markup_percent numeric,
  add column if not exists final_customer_price numeric,
  add column if not exists gross_profit numeric,
  add column if not exists gross_margin_percent numeric;

comment on column escrow_transactions.escrow_fee_payer is
  'Default must remain tradepi_globall. Escrow.com fees are internal TradePi Globall operating costs.';

comment on column escrow_transactions.escrow_fee_public_note is
  'Public text must not present escrow fee as extra customer/supplier charge.';

comment on column customer_quotes.internal_total_cost is
  'supplier_landed_cost + escrow_fee_internal + bank_transfer_fee_internal + operation_cost_internal';

comment on column customer_quotes.final_customer_price is
  'Fixed markup: internal_total_cost + markup_amount. Percent markup: internal_total_cost * (1 + markup_percent / 100).';

comment on column customer_quotes.gross_margin_percent is
  '(gross_profit / final_customer_price) * 100';
