CREATE TABLE IF NOT EXISTS game_store_metadata (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  game_project_id uuid NOT NULL,
  short_description text,
  full_description text,
  search_intent_phrases jsonb NOT NULL DEFAULT '[]'::jsonb,
  target_audience jsonb NOT NULL DEFAULT '{}'::jsonb,
  category_suggestions jsonb NOT NULL DEFAULT '[]'::jsonb,
  localization_plan jsonb NOT NULL DEFAULT '{}'::jsonb,
  play_shorts_script text,
  play_shorts_scene_plan jsonb NOT NULL DEFAULT '[]'::jsonb,
  ask_play_summary text,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);

DO $$
BEGIN
  IF EXISTS (
    SELECT 1 FROM information_schema.tables
    WHERE table_schema = 'public' AND table_name = 'game_projects'
  ) THEN
    IF NOT EXISTS (
      SELECT 1
      FROM information_schema.table_constraints
      WHERE table_schema = 'public'
        AND table_name = 'game_store_metadata'
        AND constraint_name = 'game_store_metadata_game_project_id_fkey'
    ) THEN
      ALTER TABLE game_store_metadata
        ADD CONSTRAINT game_store_metadata_game_project_id_fkey
        FOREIGN KEY (game_project_id)
        REFERENCES game_projects(id)
        ON DELETE CASCADE;
    END IF;
  END IF;
END $$;

CREATE UNIQUE INDEX IF NOT EXISTS game_store_metadata_game_project_id_uidx
  ON game_store_metadata (game_project_id);

CREATE INDEX IF NOT EXISTS game_store_metadata_created_at_idx
  ON game_store_metadata (created_at DESC);
