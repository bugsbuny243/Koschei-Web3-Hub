alter table if exists game_factory_projects
  add column if not exists title text,
  add column if not exists genre text,
  add column if not exists visual_style text,
  add column if not exists target_chain text not null default 'arbitrum-sepolia',
  add column if not exists metadata jsonb not null default '{}'::jsonb,
  add column if not exists updated_at timestamptz not null default now();

alter table if exists game_factory_projects
  alter column prompt set not null,
  alter column status set default 'draft';

alter table if exists game_factory_assets
  add column if not exists name text,
  add column if not exists description text,
  add column if not exists rarity text;

update game_factory_assets set name = coalesce(name, asset_type || '-asset');
alter table if exists game_factory_assets alter column name set not null;

alter table if exists game_factory_generated_files
  add column if not exists file_path text,
  add column if not exists metadata jsonb not null default '{}'::jsonb;

update game_factory_generated_files set file_path = coalesce(file_path, file_name, 'generated/file');
alter table if exists game_factory_generated_files alter column file_path set not null;

alter table if exists game_factory_web3_packages
  add column if not exists target_chain text default 'arbitrum-sepolia',
  add column if not exists manifest jsonb not null default '{}'::jsonb,
  add column if not exists item_schema jsonb not null default '{}'::jsonb,
  add column if not exists reward_config jsonb not null default '{}'::jsonb,
  add column if not exists adapter_config jsonb not null default '{}'::jsonb;
