CREATE TABLE IF NOT EXISTS game_projects (
  id uuid PRIMARY KEY,
  email text NOT NULL,
  title text NOT NULL,
  prompt text NOT NULL,
  target_platform text NOT NULL,
  status text NOT NULL DEFAULT 'spec_generated',
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT game_projects_target_platform_check CHECK (target_platform IN ('web_game','android_game','web_and_android'))
);

CREATE TABLE IF NOT EXISTS game_specs (
  id uuid PRIMARY KEY,
  game_project_id uuid NOT NULL REFERENCES game_projects(id) ON DELETE CASCADE,
  spec_json jsonb NOT NULL,
  summary text NOT NULL,
  status text NOT NULL DEFAULT 'spec_generated',
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_game_projects_email ON game_projects(email);
CREATE INDEX IF NOT EXISTS idx_game_specs_project_id ON game_specs(game_project_id);
