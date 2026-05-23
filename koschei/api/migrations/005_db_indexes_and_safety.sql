CREATE INDEX IF NOT EXISTS idx_payment_requests_email ON payment_requests(email);
CREATE INDEX IF NOT EXISTS idx_payment_requests_status ON payment_requests(status);
CREATE INDEX IF NOT EXISTS idx_payment_requests_plan ON payment_requests(plan);

CREATE INDEX IF NOT EXISTS idx_credits_ledger_email ON credits_ledger(email);

CREATE INDEX IF NOT EXISTS idx_runtime_projects_email ON runtime_projects(email);

CREATE INDEX IF NOT EXISTS idx_runtime_tasks_email ON runtime_tasks(email);
CREATE INDEX IF NOT EXISTS idx_runtime_tasks_project_id ON runtime_tasks(project_id);
CREATE INDEX IF NOT EXISTS idx_runtime_tasks_status ON runtime_tasks(status);

CREATE INDEX IF NOT EXISTS idx_runtime_logs_project_id ON runtime_logs(project_id);

CREATE INDEX IF NOT EXISTS idx_model_route_logs_email ON model_route_logs(email);
