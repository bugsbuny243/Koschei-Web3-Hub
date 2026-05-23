ALTER TABLE runtime_tasks
  ADD COLUMN IF NOT EXISTS input_json jsonb NOT NULL DEFAULT '{}'::jsonb,
  ADD COLUMN IF NOT EXISTS output_json jsonb NOT NULL DEFAULT '{}'::jsonb,
  ADD COLUMN IF NOT EXISTS error text,
  ADD COLUMN IF NOT EXISTS updated_at timestamptz NOT NULL DEFAULT now();

ALTER TABLE runtime_logs
  ADD COLUMN IF NOT EXISTS metadata jsonb NOT NULL DEFAULT '{}'::jsonb;

DO $$
BEGIN
  IF EXISTS (
    SELECT 1 FROM information_schema.columns
    WHERE table_schema = 'public' AND table_name = 'runtime_tasks' AND column_name = 'tool'
  ) THEN
    ALTER TABLE runtime_tasks ALTER COLUMN tool DROP NOT NULL;
  END IF;

  IF EXISTS (
    SELECT 1 FROM information_schema.columns
    WHERE table_schema = 'public' AND table_name = 'runtime_tasks' AND column_name = 'prompt'
  ) THEN
    ALTER TABLE runtime_tasks ALTER COLUMN prompt DROP NOT NULL;
  END IF;

  IF EXISTS (
    SELECT 1 FROM information_schema.columns
    WHERE table_schema = 'public' AND table_name = 'runtime_tasks' AND column_name = 'priority'
  ) THEN
    ALTER TABLE runtime_tasks ALTER COLUMN priority DROP NOT NULL;
  END IF;
END $$;
