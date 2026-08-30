-- Reports render through a template with the user's query columns mapped onto
-- its slots. Existing reports predate template selection, so they default to
-- the tabular template with an empty mapping.
ALTER TABLE reports
    ADD COLUMN IF NOT EXISTS template_id TEXT NOT NULL DEFAULT 'tabular',
    ADD COLUMN IF NOT EXISTS template_config JSONB NOT NULL DEFAULT '{}'::jsonb;
