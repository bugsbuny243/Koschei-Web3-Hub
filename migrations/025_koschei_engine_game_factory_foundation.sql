CREATE TABLE IF NOT EXISTS game_projects (
  id uuid PRIMARY KEY,
  user_id text NOT NULL,
  title text NOT NULL,
  slug text,
  prompt text NOT NULL,
  game_type text NOT NULL DEFAULT 'unknown',
  target_platform text NOT NULL DEFAULT 'web_and_android',
  ownership_status text NOT NULL DEFAULT 'customer_owned',
  status text NOT NULL DEFAULT 'draft',
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS game_specs (
  id uuid PRIMARY KEY,
  game_project_id uuid NOT NULL REFERENCES game_projects(id) ON DELETE CASCADE,
  spec_json jsonb NOT NULL DEFAULT '{}'::jsonb,
  generated_by_model text,
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS game_build_jobs (
  id uuid PRIMARY KEY,
  game_project_id uuid NOT NULL REFERENCES game_projects(id) ON DELETE CASCADE,
  target_platform text NOT NULL,
  status text NOT NULL DEFAULT 'queued',
  logs text,
  error_message text,
  started_at timestamptz,
  finished_at timestamptz,
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS game_artifacts (
  id uuid PRIMARY KEY,
  game_project_id uuid NOT NULL REFERENCES game_projects(id) ON DELETE CASCADE,
  build_job_id uuid REFERENCES game_build_jobs(id) ON DELETE SET NULL,
  artifact_type text NOT NULL,
  file_name text,
  file_url text,
  file_size_mb numeric DEFAULT 0,
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS google_play_integrations (
  id uuid PRIMARY KEY,
  user_id text NOT NULL,
  app_package_name text NOT NULL,
  service_account_json_encrypted text,
  status text NOT NULL DEFAULT 'connected',
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS production_release_jobs (
  id uuid PRIMARY KEY,
  game_project_id uuid NOT NULL REFERENCES game_projects(id) ON DELETE CASCADE,
  artifact_id uuid REFERENCES game_artifacts(id) ON DELETE SET NULL,
  google_play_integration_id uuid REFERENCES google_play_integrations(id) ON DELETE SET NULL,
  release_track text NOT NULL DEFAULT 'production',
  status text NOT NULL DEFAULT 'queued',
  google_edit_id text,
  error_message text,
  submitted_at timestamptz,
  completed_at timestamptz,
  created_at timestamptz NOT NULL DEFAULT now()
);

DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM pg_constraint WHERE conname = 'game_projects_target_platform_check'
  ) THEN
    ALTER TABLE game_projects
      ADD CONSTRAINT game_projects_target_platform_check
      CHECK (target_platform IN ('web_game', 'android_game', 'web_and_android'));
  END IF;
END $$;

DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM pg_constraint WHERE conname = 'game_build_jobs_status_check'
  ) THEN
    ALTER TABLE game_build_jobs
      ADD CONSTRAINT game_build_jobs_status_check
      CHECK (status IN ('queued', 'building', 'packaged', 'failed'));
  END IF;
END $$;

DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM pg_constraint WHERE conname = 'production_release_jobs_release_track_check'
  ) THEN
    ALTER TABLE production_release_jobs
      ADD CONSTRAINT production_release_jobs_release_track_check
      CHECK (release_track IN ('production', 'internal', 'closed', 'open'));
  END IF;
END $$;

DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM pg_constraint WHERE conname = 'production_release_jobs_status_check'
  ) THEN
    ALTER TABLE production_release_jobs
      ADD CONSTRAINT production_release_jobs_status_check
      CHECK (status IN ('queued', 'uploading', 'submitted', 'published', 'failed'));
  END IF;
END $$;
