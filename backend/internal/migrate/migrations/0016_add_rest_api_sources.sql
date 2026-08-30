ALTER TABLE data_sources
    ADD COLUMN rest_url TEXT,
    ADD COLUMN rest_method TEXT,
    ADD COLUMN rest_headers TEXT,
    ADD COLUMN rest_auth_type TEXT,
    ADD COLUMN rest_encrypted_auth_config BYTEA,
    ADD COLUMN rest_body TEXT;
