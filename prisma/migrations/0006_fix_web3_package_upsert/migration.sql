alter table if exists game_factory_web3_packages
  add column if not exists updated_at timestamptz default now();

create unique index if not exists uq_game_factory_web3_packages_project_id
  on game_factory_web3_packages(project_id);
