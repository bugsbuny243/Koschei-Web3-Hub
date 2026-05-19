create table if not exists game_factory_projects (
  id uuid primary key default gen_random_uuid(),
  name text not null,
  slug text not null unique,
  prompt text not null,
  status text not null default 'draft',
  game_brief jsonb,
  phaser_template text,
  extracted_items jsonb not null default '[]'::jsonb,
  created_at timestamptz not null default now()
);

create table if not exists game_factory_briefs (
  id uuid primary key default gen_random_uuid(),
  project_id uuid not null references game_factory_projects(id) on delete cascade,
  prompt text not null,
  brief jsonb not null,
  created_at timestamptz not null default now()
);

create table if not exists game_factory_assets (
  id uuid primary key default gen_random_uuid(),
  project_id uuid not null references game_factory_projects(id) on delete cascade,
  asset_type text not null,
  path text,
  metadata jsonb not null default '{}'::jsonb,
  created_at timestamptz not null default now()
);

create table if not exists game_factory_generated_files (
  id uuid primary key default gen_random_uuid(),
  project_id uuid not null references game_factory_projects(id) on delete cascade,
  file_type text not null,
  file_name text not null,
  content text not null,
  created_at timestamptz not null default now()
);

create table if not exists game_factory_web3_packages (
  id uuid primary key default gen_random_uuid(),
  project_id uuid not null references game_factory_projects(id) on delete cascade,
  chain_slug text not null,
  nft_metadata jsonb not null,
  bridge_config jsonb not null,
  export_bundle jsonb not null,
  created_at timestamptz not null default now()
);
