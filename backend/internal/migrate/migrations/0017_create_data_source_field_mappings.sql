-- Maps a data source's detected API fields onto DocuWave's predefined system
-- fields, so a report can be built against a stable set of names regardless
-- of what the upstream API happens to call things.
CREATE TABLE IF NOT EXISTS data_source_field_mappings (
    data_source_id UUID PRIMARY KEY REFERENCES data_sources(id) ON DELETE CASCADE,
    mapping JSONB NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
