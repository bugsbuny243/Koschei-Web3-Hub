CREATE TABLE IF NOT EXISTS runtime_projects (
  id UUID PRIMARY KEY,
  email TEXT NOT NULL,
  title TEXT NOT NULL,
  prompt TEXT NOT NULL,
  status TEXT NOT NULL DEFAULT 'queued',
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS runtime_tasks (
  id UUID PRIMARY KEY,
  project_id UUID REFERENCES runtime_projects(id),
  email TEXT NOT NULL,
  task_type TEXT NOT NULL,
  tool TEXT NOT NULL,
  prompt TEXT NOT NULL,
  status TEXT NOT NULL DEFAULT 'queued',
  priority INTEGER NOT NULL DEFAULT 5,
  result TEXT,
  error TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  started_at TIMESTAMPTZ,
  completed_at TIMESTAMPTZ,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS runtime_logs (
  id UUID PRIMARY KEY,
  project_id UUID,
  task_id UUID,
  level TEXT NOT NULL DEFAULT 'info',
  message TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_runtime_projects_email ON runtime_projects(email);
CREATE INDEX IF NOT EXISTS idx_runtime_tasks_email ON runtime_tasks(email);
CREATE INDEX IF NOT EXISTS idx_runtime_tasks_project_id ON runtime_tasks(project_id);
CREATE INDEX IF NOT EXISTS idx_runtime_logs_project_id ON runtime_logs(project_id);
