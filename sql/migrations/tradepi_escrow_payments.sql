create table if not exists escrow_transactions (
  id uuid primary key default gen_random_uuid(),
  quote_request_id uuid references quote_requests(id),
  customer_quote_id uuid references customer_quotes(id),
  escrow_transaction_id text unique,
  escrow_status text default 'draft',
  escrow_event_status text,
  buyer_email text,
  seller_email text,
  currency text default 'usd',
  item_title text,
  item_description text,
  item_category text default 'heavy_equipment_and_machinery',
  final_customer_price numeric not null,
  escrow_fee_payer text default 'buyer',
  escrow_fee_estimate numeric,
  payment_link text,
  raw_create_payload jsonb default '{}'::jsonb,
  raw_response jsonb default '{}'::jsonb,
  created_by text,
  created_at timestamptz default now(),
  updated_at timestamptz default now()
);

create table if not exists escrow_webhook_events (
  id uuid primary key default gen_random_uuid(),
  escrow_transaction_id text,
  event text,
  event_type text,
  raw_payload jsonb not null,
  verified_by_fetch boolean default false,
  received_at timestamptz default now()
);

create table if not exists supplier_payments (
  id uuid primary key default gen_random_uuid(),
  quote_request_id uuid references quote_requests(id),
  customer_quote_id uuid references customer_quotes(id),
  supplier_name text default 'Kaifeng Lecheng Machinery Co., Ltd.',
  supplier_contact_name text default 'Cathy',
  payment_stage text not null,
  payment_method text default 'T/T bank transfer',
  currency text default 'USD',
  amount numeric,
  percent_of_supplier_cost numeric,
  due_event text,
  status text default 'pending',
  paid_at timestamptz,
  proof_file_url text,
  private_notes text,
  created_at timestamptz default now(),
  updated_at timestamptz default now()
);

create table if not exists payment_audit_logs (
  id uuid primary key default gen_random_uuid(),
  quote_request_id uuid,
  customer_quote_id uuid,
  actor text,
  action text not null,
  details jsonb default '{}'::jsonb,
  created_at timestamptz default now()
);
