-- A unified per-user overlay of archived templates, applying to both
-- built-in templates (which aren't rows a user owns) and custom templates
-- (which are). Archiving hides a template from a user's picker without
-- deleting it or affecting any other account, and without breaking reports
-- that already reference it.
CREATE TABLE IF NOT EXISTS template_archive_state (
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    template_id TEXT NOT NULL,
    archived_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, template_id)
);
