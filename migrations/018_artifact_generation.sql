CREATE TABLE IF NOT EXISTS generated_artifacts (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  runtime_project_id uuid NOT NULL,
  user_email text,
  status text NOT NULL DEFAULT 'queued',
  artifact_type text NOT NULL DEFAULT 'code_package',
  title text,
  summary text,
  file_count integer DEFAULT 0,
  zip_ready boolean DEFAULT false,
  error_message text,
  metadata jsonb NOT NULL DEFAULT '{}'::jsonb,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS generated_files (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  artifact_id uuid NOT NULL REFERENCES generated_artifacts(id) ON DELETE CASCADE,
  runtime_project_id uuid NOT NULL,
  path text NOT NULL,
  language text,
  purpose text,
  action text,
  content text NOT NULL,
  content_hash text,
  metadata jsonb NOT NULL DEFAULT '{}'::jsonb,
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_generated_artifacts_runtime_project_id ON generated_artifacts(runtime_project_id);
CREATE INDEX IF NOT EXISTS idx_generated_files_artifact_id ON generated_files(artifact_id);
CREATE INDEX IF NOT EXISTS idx_generated_files_runtime_project_id ON generated_files(runtime_project_id);
