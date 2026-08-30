-- A custom template is a user's own block-composed report design: an ordered
-- list of blocks (type + declared slots), kept as a live definition every
-- report referencing it renders through. No versioning — editing it changes
-- every report that uses it, going forward.
CREATE TABLE IF NOT EXISTS custom_templates (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    blocks JSONB NOT NULL DEFAULT '[]'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_custom_templates_user_id ON custom_templates (user_id);
