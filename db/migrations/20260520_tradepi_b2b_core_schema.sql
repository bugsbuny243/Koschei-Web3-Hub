-- TradePi Globall B2B core schema
-- Idempotent migration

CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE IF NOT EXISTS products (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  slug text UNIQUE NOT NULL,
  name text NOT NULL,
  model text,
  category text,
  short_description text,
  long_description text,
  is_public boolean DEFAULT false,
  status text DEFAULT 'active',
  created_at timestamptz DEFAULT now(),
  updated_at timestamptz DEFAULT now()
);

CREATE TABLE IF NOT EXISTS product_media (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  product_id uuid REFERENCES products(id) ON DELETE CASCADE,
  media_type text,
  title text,
  file_path text NOT NULL,
  alt_text text,
  sort_order integer DEFAULT 0,
  is_primary boolean DEFAULT false,
  created_at timestamptz DEFAULT now()
);

CREATE TABLE IF NOT EXISTS suppliers (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  name text NOT NULL,
  country text,
  city text,
  website text,
  notes text,
  is_active boolean DEFAULT true,
  created_at timestamptz DEFAULT now()
);

CREATE TABLE IF NOT EXISTS supplier_contacts (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  supplier_id uuid REFERENCES suppliers(id) ON DELETE CASCADE,
  contact_name text NOT NULL,
  role text,
  email text,
  whatsapp text,
  wechat text,
  notes text,
  is_primary boolean DEFAULT false,
  created_at timestamptz DEFAULT now()
);

CREATE TABLE IF NOT EXISTS quote_requests (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  status text DEFAULT 'new',
  full_name text,
  company_name text,
  company_registration_status text,
  tax_number text,
  tax_office text,
  company_address text,
  email text,
  phone text,
  whatsapp text,
  country text,
  city text,
  district text,
  full_delivery_address text,
  delivery_contact_name text,
  delivery_contact_phone text,
  business_type text,
  main_agricultural_activity text,
  crop_types text,
  current_processing_capacity text,
  required_capacity_tph text,
  expected_daily_volume text,
  impurity_problem_description text,
  target_cleaning_result text,
  product_interest text,
  required_configuration_notes text,
  need_control_cabinet boolean DEFAULT false,
  need_fan_cyclone boolean DEFAULT false,
  need_bucket_elevator boolean DEFAULT false,
  need_spare_screen_sets boolean DEFAULT false,
  requested_screen_sets text,
  voltage_available text,
  installation_location_type text,
  forklift_or_unloading_available boolean DEFAULT false,
  expected_purchase_time text,
  quantity integer DEFAULT 1,
  message text,
  company_info_complete boolean DEFAULT false,
  importer_info_warning text,
  missing_fields_json jsonb DEFAULT '{}'::jsonb,
  created_at timestamptz DEFAULT now(),
  updated_at timestamptz DEFAULT now()
);

CREATE TABLE IF NOT EXISTS rfq_ai_reports (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  quote_request_id uuid REFERENCES quote_requests(id) ON DELETE CASCADE,
  model_provider text,
  model_name text,
  rfq_summary_json jsonb DEFAULT '{}'::jsonb,
  missing_fields_json jsonb DEFAULT '{}'::jsonb,
  risk_notes text,
  supplier_message_en text,
  admin_notes text,
  created_at timestamptz DEFAULT now()
);

CREATE TABLE IF NOT EXISTS supplier_messages (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  quote_request_id uuid REFERENCES quote_requests(id) ON DELETE CASCADE,
  supplier_id uuid REFERENCES suppliers(id),
  contact_id uuid REFERENCES supplier_contacts(id),
  subject text,
  message_body text,
  language text DEFAULT 'en',
  channel text DEFAULT 'manual',
  status text DEFAULT 'draft',
  sent_at timestamptz,
  created_at timestamptz DEFAULT now()
);

CREATE TABLE IF NOT EXISTS supplier_ddp_quotes (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  quote_request_id uuid REFERENCES quote_requests(id) ON DELETE CASCADE,
  supplier_id uuid REFERENCES suppliers(id),
  contact_id uuid REFERENCES supplier_contacts(id),
  product_name text,
  product_model text,
  supplier_factory_price_usd numeric,
  supplier_ddp_all_in_price_usd numeric,
  included_items_json jsonb DEFAULT '{}'::jsonb,
  ddp_scope_notes text,
  insurance_scope_notes text,
  supplier_risk_scope_notes text,
  required_buyer_documents text,
  production_time_working_days integer DEFAULT 35,
  production_time_calendar_estimate_days integer DEFAULT 50,
  estimated_total_delivery_min_days integer DEFAULT 75,
  estimated_total_delivery_max_days integer DEFAULT 80,
  shipping_time_notes text,
  customs_time_notes text,
  delay_disclaimer text,
  quote_valid_until date,
  raw_supplier_response text,
  status text DEFAULT 'received',
  created_at timestamptz DEFAULT now(),
  updated_at timestamptz DEFAULT now()
);

CREATE TABLE IF NOT EXISTS customer_final_quotes (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  quote_request_id uuid REFERENCES quote_requests(id) ON DELETE CASCADE,
  supplier_ddp_quote_id uuid REFERENCES supplier_ddp_quotes(id),
  supplier_ddp_all_in_price_usd numeric NOT NULL,
  escrow_fee_internal_usd numeric DEFAULT 0,
  bank_fee_internal_usd numeric DEFAULT 0,
  operation_cost_internal_usd numeric DEFAULT 0,
  commission_type text DEFAULT 'percent',
  commission_percent numeric DEFAULT 0,
  commission_fixed_usd numeric DEFAULT 0,
  commission_amount_usd numeric DEFAULT 0,
  final_customer_price_usd numeric,
  currency text DEFAULT 'USD',
  gross_profit_usd numeric,
  gross_margin_percent numeric,
  payment_terms text,
  delivery_terms text DEFAULT 'Supplier-confirmed DDP door-to-door',
  estimated_delivery_min_days integer DEFAULT 75,
  estimated_delivery_max_days integer DEFAULT 80,
  customer_visible_notes text,
  internal_notes text,
  status text DEFAULT 'draft',
  created_at timestamptz DEFAULT now(),
  updated_at timestamptz DEFAULT now()
);

CREATE TABLE IF NOT EXISTS escrow_transactions (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  quote_request_id uuid REFERENCES quote_requests(id) ON DELETE CASCADE,
  customer_final_quote_id uuid REFERENCES customer_final_quotes(id),
  escrow_transaction_id text,
  escrow_status text,
  escrow_env text,
  currency text DEFAULT 'USD',
  amount_usd numeric,
  buyer_email text,
  seller_email text,
  escrow_fee_payer text DEFAULT 'tradepi_globall',
  escrow_fee_paid_by_tradepi boolean DEFAULT true,
  payment_link text,
  raw_request_json jsonb DEFAULT '{}'::jsonb,
  raw_response_json jsonb DEFAULT '{}'::jsonb,
  created_at timestamptz DEFAULT now(),
  updated_at timestamptz DEFAULT now()
);

CREATE TABLE IF NOT EXISTS escrow_webhook_events (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  escrow_transaction_id uuid REFERENCES escrow_transactions(id) ON DELETE CASCADE,
  event_type text,
  raw_payload_json jsonb DEFAULT '{}'::jsonb,
  verified_by_fetch boolean DEFAULT false,
  processed_at timestamptz,
  created_at timestamptz DEFAULT now()
);

CREATE TABLE IF NOT EXISTS supplier_payment_milestones (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  quote_request_id uuid REFERENCES quote_requests(id) ON DELETE CASCADE,
  supplier_ddp_quote_id uuid REFERENCES supplier_ddp_quotes(id),
  milestone_type text,
  percentage numeric,
  amount_usd numeric,
  due_condition text,
  status text DEFAULT 'pending',
  payment_method text,
  paid_at timestamptz,
  proof_file_url text,
  supplier_confirmation_notes text,
  internal_notes text,
  created_at timestamptz DEFAULT now(),
  updated_at timestamptz DEFAULT now()
);

CREATE TABLE IF NOT EXISTS documents (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  quote_request_id uuid REFERENCES quote_requests(id) ON DELETE CASCADE,
  related_table text,
  related_id uuid,
  document_type text,
  title text,
  file_url text,
  file_path text,
  visibility text DEFAULT 'internal',
  notes text,
  created_at timestamptz DEFAULT now()
);

CREATE TABLE IF NOT EXISTS audit_logs (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  actor_type text,
  actor_id text,
  action text,
  entity_type text,
  entity_id uuid,
  before_json jsonb DEFAULT '{}'::jsonb,
  after_json jsonb DEFAULT '{}'::jsonb,
  created_at timestamptz DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_quote_requests_status ON quote_requests(status);
CREATE INDEX IF NOT EXISTS idx_quote_requests_email ON quote_requests(email);
CREATE INDEX IF NOT EXISTS idx_supplier_ddp_quotes_quote_request_id ON supplier_ddp_quotes(quote_request_id);
CREATE INDEX IF NOT EXISTS idx_customer_final_quotes_quote_request_id ON customer_final_quotes(quote_request_id);
CREATE INDEX IF NOT EXISTS idx_escrow_transactions_quote_request_id ON escrow_transactions(quote_request_id);
CREATE INDEX IF NOT EXISTS idx_supplier_payment_milestones_quote_request_id ON supplier_payment_milestones(quote_request_id);
CREATE INDEX IF NOT EXISTS idx_product_media_product_id ON product_media(product_id);

INSERT INTO suppliers (name)
SELECT 'Kaifeng Lecheng Machinery'
WHERE NOT EXISTS (
  SELECT 1 FROM suppliers WHERE name = 'Kaifeng Lecheng Machinery'
);

INSERT INTO supplier_contacts (supplier_id, contact_name, is_primary)
SELECT s.id, 'Cathy', true
FROM suppliers s
WHERE s.name = 'Kaifeng Lecheng Machinery'
  AND NOT EXISTS (
    SELECT 1
    FROM supplier_contacts c
    WHERE c.supplier_id = s.id
      AND c.contact_name = 'Cathy'
  );

INSERT INTO products (slug, name, model, is_public)
SELECT 'fine-cleaner-5x-5', 'Fine Cleaner 5X-5', '5X-5', false
WHERE NOT EXISTS (
  SELECT 1 FROM products WHERE slug = 'fine-cleaner-5x-5'
);
