create extension if not exists pgcrypto;

create table if not exists suppliers (
  id uuid primary key default gen_random_uuid(),
  name text not null,
  country text,
  contact_name text,
  contact_email text,
  phone text,
  website text,
  notes text,
  created_at timestamptz default now()
);
create table if not exists product_categories (
  id uuid primary key default gen_random_uuid(),
  slug text unique not null,
  name text not null,
  description text,
  sort_order integer default 0,
  created_at timestamptz default now()
);
create table if not exists products (
  id uuid primary key default gen_random_uuid(),
  supplier_id uuid references suppliers(id),
  category_id uuid references product_categories(id),
  slug text unique not null,
  name text not null,
  model text,
  short_description text,
  long_description text,
  applications jsonb default '[]'::jsonb,
  specs jsonb default '{}'::jsonb,
  internal_costs jsonb default '{}'::jsonb,
  public_price_mode text default 'request_quote',
  is_active boolean default true,
  created_at timestamptz default now(),
  updated_at timestamptz default now()
);
create table if not exists product_documents (
  id uuid primary key default gen_random_uuid(),
  product_id uuid references products(id),
  title text not null,
  document_type text,
  file_url text,
  is_public boolean default false,
  created_at timestamptz default now()
);
create table if not exists quote_requests (
  id uuid primary key default gen_random_uuid(),
  full_name text not null,
  company_name text,
  email text not null,
  phone text,
  country text,
  city text,
  product_interest text,
  raw_material_type text,
  required_capacity_tph text,
  target_quantity text,
  preferred_trade_term text,
  destination_port_or_city text,
  message text,
  status text default 'new',
  admin_notes text,
  metadata jsonb default '{}'::jsonb,
  created_at timestamptz default now(),
  updated_at timestamptz default now()
);
create table if not exists supplier_quotes (
  id uuid primary key default gen_random_uuid(),
  quote_request_id uuid references quote_requests(id),
  supplier_id uuid references suppliers(id),
  trade_term text,
  currency text default 'USD',
  exw_amount numeric,
  fob_amount numeric,
  cif_amount numeric,
  ddp_amount numeric,
  destination text,
  validity_date date,
  delivery_time text,
  payment_terms text,
  notes text,
  created_at timestamptz default now()
);

insert into suppliers (name,country,notes)
values ('Kaifeng Lecheng Machinery Co., Ltd.','China','Seed data for TradePi Globall Machinery')
on conflict do nothing;

insert into product_categories (slug,name,description,sort_order)
values ('grain-cleaning-machines','Grain Cleaning Machines','Cleaning and pre-cleaning equipment for grains.',1)
on conflict (slug) do nothing;

with s as (select id from suppliers where name='Kaifeng Lecheng Machinery Co., Ltd.' limit 1),
 c as (select id from product_categories where slug='grain-cleaning-machines' limit 1)
insert into products (supplier_id,category_id,slug,name,model,short_description,long_description,applications,specs,internal_costs,public_price_mode)
select s.id,c.id,'fine-cleaner-5x-5','Fine Cleaner 5X-5','5X-5',
'The 5X Air Screen Fine Cleaner is used for pre-cleaning and intensive cleaning of grains and seeds.',
'Removes dust, light impurities, big and small impurities, and part of infected or broken seeds.',
'["wheat","soybean","beans","seeds","grain cleaning"]'::jsonb,
'{"capacity":"5 TPH based on wheat","size":"3200 x 1940 x 3600 mm","weight":"3250 kg","power":"6.7 KW","screens":"4 layers / 7 pieces","hs_code":"8437109"}'::jsonb,
'{"EXW_USD":20560,"FOB_USD":21860,"CIF_40HQ_Trabzon_USD":27560,"DDP_40HQ_Erzincan_USD":30060,"payment_terms":"T/T 30% prepaid, 70% balance before delivery","delivery":"35 working days after prepayment"}'::jsonb,
'request_quote'
from s,c
on conflict (slug) do nothing;
