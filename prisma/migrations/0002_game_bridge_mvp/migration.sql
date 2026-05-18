create table if not exists game_bridge_projects (
  id uuid primary key default gen_random_uuid(),
  name text not null,
  slug text not null unique,
  genre text,
  target_chain text,
  status text not null default 'draft',
  description text,
  created_at timestamptz not null default now()
);

create table if not exists game_bridge_items (
  id uuid primary key default gen_random_uuid(),
  project_id uuid references game_bridge_projects(id) on delete set null,
  item_key text not null unique,
  name text not null,
  item_type text not null,
  rarity text not null,
  image_uri text,
  attributes jsonb not null default '{}'::jsonb,
  created_at timestamptz not null default now()
);

create table if not exists game_bridge_metadata (
  id uuid primary key default gen_random_uuid(),
  item_id uuid references game_bridge_items(id) on delete cascade,
  metadata jsonb not null,
  schema_version text not null default '1.0',
  created_at timestamptz not null default now()
);

create table if not exists game_bridge_adapters (
  id uuid primary key default gen_random_uuid(),
  project_id uuid references game_bridge_projects(id) on delete cascade,
  adapter_name text not null,
  chain_slug text not null,
  config jsonb not null,
  created_at timestamptz not null default now()
);

create table if not exists game_bridge_ai_outputs (
  id uuid primary key default gen_random_uuid(),
  project_id uuid references game_bridge_projects(id) on delete set null,
  item_id uuid references game_bridge_items(id) on delete set null,
  output_type text not null,
  prompt text,
  output_text text,
  output_json jsonb,
  created_at timestamptz not null default now()
);
