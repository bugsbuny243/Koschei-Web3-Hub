create table if not exists supplier_terms (
  id uuid primary key default gen_random_uuid(),
  supplier_id uuid references suppliers(id),
  product_slug text,
  payment_terms text,
  deposit_percent numeric,
  balance_percent numeric,
  balance_due_event text,
  voltage text,
  freight_note text,
  validity_note text,
  private_notes text,
  is_public boolean default false,
  created_at timestamptz default now(),
  updated_at timestamptz default now()
);

create table if not exists supplier_conversation_notes (
  id uuid primary key default gen_random_uuid(),
  supplier_id uuid references suppliers(id),
  product_slug text,
  channel text,
  contact_name text,
  summary text,
  extracted_requirements jsonb default '{}'::jsonb,
  private_note text,
  is_public boolean default false,
  created_at timestamptz default now()
);

insert into supplier_terms (
  product_slug,
  payment_terms,
  deposit_percent,
  balance_percent,
  balance_due_event,
  voltage,
  freight_note,
  validity_note,
  is_public
) values (
  'fine-cleaner-5x-5',
  'T/T 30% advance payment, 70% balance before delivery',
  30,
  70,
  'before delivery',
  '380V 50Hz 3 Phase',
  'DDP / door-to-door Erzincan was discussed, but freight is not fixed and must be recalculated at actual shipping date.',
  'Freight and final quotation terms must be checked again before final order.',
  false
);

insert into supplier_conversation_notes (
  product_slug,
  channel,
  contact_name,
  summary,
  extracted_requirements,
  is_public
) values (
  'fine-cleaner-5x-5',
  'Made-in-China + WhatsApp',
  'Cathy',
  'Initial supplier discussion for Fine Cleaner 5X-5 and related configuration. Buyer asked whether the machine can process white beans, wheat, barley, alfalfa seed and cumin by changing screens. Supplier indicated grain cleaning machine can process these crops by changing screens and discussed fine cleaner configuration, control cabinet, fan, cyclone dust collection, low-speed bucket elevator, screen sets, 380V 50Hz 3 phase compatibility, and DDP/door-to-door Erzincan freight inquiry.',
  '{"target_crops":["white bean","wheat","barley","alfalfa seed","cumin"],"machine":"Fine Cleaner 5X-5","needs_control_cabinet":true,"needs_fan":true,"needs_cyclone":true,"needs_bucket_elevator":true,"bucket_elevator_note":"low-speed / anti-broken requested","screen_sets":["wheat","barley","white bean"],"voltage":"380V 50Hz 3 Phase","delivery_request":"DDP / door-to-door Erzincan, Turkey","freight_warning":"Freight is not fixed and must be recalculated at actual shipment date","payment_terms":"T/T 30% advance payment, 70% balance before delivery"}'::jsonb,
  false
);
