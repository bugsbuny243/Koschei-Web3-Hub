alter table if exists game_factory_projects
  add column if not exists title text,
  add column if not exists prompt text,
  add column if not exists genre text,
  add column if not exists visual_style text,
  add column if not exists target_chain text default 'arbitrum-sepolia',
  add column if not exists status text default 'draft',
  add column if not exists metadata jsonb default '{}'::jsonb,
  add column if not exists updated_at timestamptz default now();

update game_factory_projects set prompt = coalesce(prompt, '');
update game_factory_projects set target_chain = coalesce(target_chain, 'arbitrum-sepolia');
update game_factory_projects set status = coalesce(status, 'draft');
update game_factory_projects set metadata = coalesce(metadata, '{}'::jsonb);
update game_factory_projects set updated_at = coalesce(updated_at, now());
alter table if exists game_factory_projects alter column prompt set not null;
alter table if exists game_factory_projects alter column target_chain set default 'arbitrum-sepolia';
alter table if exists game_factory_projects alter column status set default 'draft';
alter table if exists game_factory_projects alter column metadata set default '{}'::jsonb;
alter table if exists game_factory_projects alter column updated_at set default now();
alter table if exists game_factory_projects alter column name drop not null;
alter table if exists game_factory_projects alter column slug drop not null;

alter table if exists game_factory_briefs
  add column if not exists project_id uuid,
  add column if not exists brief jsonb default '{}'::jsonb;
update game_factory_briefs set brief = coalesce(brief, '{}'::jsonb);
alter table if exists game_factory_briefs alter column brief set default '{}'::jsonb;
alter table if exists game_factory_briefs alter column prompt drop not null;

alter table if exists game_factory_assets
  add column if not exists asset_type text,
  add column if not exists name text,
  add column if not exists description text,
  add column if not exists rarity text,
  add column if not exists metadata jsonb default '{}'::jsonb;
update game_factory_assets set asset_type = coalesce(asset_type, 'item');
update game_factory_assets set name = coalesce(name, asset_type || '-asset');
update game_factory_assets set description = coalesce(description, '');
update game_factory_assets set rarity = coalesce(rarity, 'common');
update game_factory_assets set metadata = coalesce(metadata, '{}'::jsonb);
alter table if exists game_factory_assets alter column asset_type set not null;
alter table if exists game_factory_assets alter column name set not null;
alter table if exists game_factory_assets alter column description set not null;
alter table if exists game_factory_assets alter column rarity set not null;
alter table if exists game_factory_assets alter column metadata set default '{}'::jsonb;

alter table if exists game_factory_generated_files
  add column if not exists file_path text,
  add column if not exists file_type text,
  add column if not exists content text,
  add column if not exists metadata jsonb default '{}'::jsonb;
update game_factory_generated_files set file_path = coalesce(file_path, file_name, 'generated/file');
update game_factory_generated_files set file_type = coalesce(file_type, 'text');
update game_factory_generated_files set content = coalesce(content, '');
update game_factory_generated_files set metadata = coalesce(metadata, '{}'::jsonb);
alter table if exists game_factory_generated_files alter column file_path set not null;
alter table if exists game_factory_generated_files alter column file_type set not null;
alter table if exists game_factory_generated_files alter column content set not null;
alter table if exists game_factory_generated_files alter column metadata set default '{}'::jsonb;
alter table if exists game_factory_generated_files alter column file_name drop not null;

alter table if exists game_factory_web3_packages
  add column if not exists target_chain text default 'arbitrum-sepolia',
  add column if not exists manifest jsonb default '{}'::jsonb,
  add column if not exists item_schema jsonb default '{}'::jsonb,
  add column if not exists nft_metadata jsonb default '{}'::jsonb,
  add column if not exists reward_config jsonb default '{}'::jsonb,
  add column if not exists adapter_config jsonb default '{}'::jsonb;
update game_factory_web3_packages set target_chain = coalesce(target_chain, 'arbitrum-sepolia');
update game_factory_web3_packages set manifest = coalesce(manifest, '{}'::jsonb);
update game_factory_web3_packages set item_schema = coalesce(item_schema, '{}'::jsonb);
update game_factory_web3_packages set nft_metadata = coalesce(nft_metadata, '{}'::jsonb);
update game_factory_web3_packages set reward_config = coalesce(reward_config, '{}'::jsonb);
update game_factory_web3_packages set adapter_config = coalesce(adapter_config, '{}'::jsonb);
alter table if exists game_factory_web3_packages alter column target_chain set default 'arbitrum-sepolia';
alter table if exists game_factory_web3_packages alter column manifest set default '{}'::jsonb;
alter table if exists game_factory_web3_packages alter column item_schema set default '{}'::jsonb;
alter table if exists game_factory_web3_packages alter column nft_metadata set default '{}'::jsonb;
alter table if exists game_factory_web3_packages alter column reward_config set default '{}'::jsonb;
alter table if exists game_factory_web3_packages alter column adapter_config set default '{}'::jsonb;
alter table if exists game_factory_web3_packages alter column chain_slug drop not null;
alter table if exists game_factory_web3_packages alter column bridge_config drop not null;
alter table if exists game_factory_web3_packages alter column export_bundle drop not null;
