CREATE TABLE IF NOT EXISTS google_sheets_connections (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    encrypted_token BYTEA NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_google_sheets_connections_user_id ON google_sheets_connections (user_id);

ALTER TABLE data_sources
    ALTER COLUMN host DROP NOT NULL,
    ALTER COLUMN port DROP NOT NULL,
    ALTER COLUMN db_name DROP NOT NULL,
    ALTER COLUMN username DROP NOT NULL,
    ALTER COLUMN encrypted_password DROP NOT NULL,
    ADD COLUMN spreadsheet_id TEXT,
    ADD COLUMN spreadsheet_name TEXT,
    ADD COLUMN google_connection_id UUID REFERENCES google_sheets_connections(id) ON DELETE CASCADE;
