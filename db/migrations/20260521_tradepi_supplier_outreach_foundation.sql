create table if not exists supplier_discovery_jobs (
  id uuid primary key default gen_random_uuid(),
  product_category text,
  keywords text,
  target_country text default 'China',
  target_platform text,
  status text default 'pending',
  search_query text,
  created_by text,
  started_at timestamptz,
  finished_at timestamptz,
  error_message text,
  created_at timestamptz default now(),
  updated_at timestamptz default now()
);

create table if not exists supplier_leads (
  id uuid primary key default gen_random_uuid(),
  discovery_job_id uuid references supplier_discovery_jobs(id) on delete set null,
  company_name text,
  platform text,
  source_url text not null,
  country text,
  city text,
  product_categories text[],
  is_verified_claimed boolean default false,
  likely_manufacturer boolean default false,
  likely_trader boolean default false,
  manufacturer_score numeric,
  risk_score numeric,
  confidence text default 'low',
  status text default 'new',
  contact_email text,
  contact_whatsapp text,
  contact_page_url text,
  notes text,
  created_at timestamptz default now(),
  updated_at timestamptz default now()
);

create table if not exists supplier_lead_sources (
  id uuid primary key default gen_random_uuid(),
  supplier_lead_id uuid references supplier_leads(id) on delete cascade,
  source_title text,
  source_url text not null,
  source_snippet text,
  search_query text,
  platform text,
  created_at timestamptz default now()
);

create table if not exists supplier_ai_analyses (
  id uuid primary key default gen_random_uuid(),
  supplier_lead_id uuid references supplier_leads(id) on delete cascade,
  raw_ai_output jsonb default '{}'::jsonb,
  likely_manufacturer boolean,
  likely_trader boolean,
  verified_claim_found boolean,
  product_fit text,
  manufacturer_score numeric,
  risk_score numeric,
  contact_possible boolean,
  contact_method text,
  risk_notes text,
  recommended_action text,
  confidence text default 'low',
  created_at timestamptz default now()
);

create table if not exists supplier_outreach_messages (
  id uuid primary key default gen_random_uuid(),
  supplier_lead_id uuid references supplier_leads(id) on delete cascade,
  language text default 'en',
  subject text,
  body text not null,
  status text default 'draft',
  approved_by_owner boolean default false,
  sent_at timestamptz,
  send_method text,
  created_at timestamptz default now(),
  updated_at timestamptz default now()
);

create table if not exists supplier_outreach_events (
  id uuid primary key default gen_random_uuid(),
  supplier_lead_id uuid references supplier_leads(id) on delete cascade,
  event_type text not null,
  note text,
  created_by text,
  created_at timestamptz default now()
);
