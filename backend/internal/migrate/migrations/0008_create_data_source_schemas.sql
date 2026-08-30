-- A data source's structure is read once when it's connected and kept, so
-- building a report's query is a local operation against known tables and
-- columns instead of a round trip to the user's database.
CREATE TABLE IF NOT EXISTS data_source_schemas (
    data_source_id UUID PRIMARY KEY REFERENCES data_sources(id) ON DELETE CASCADE,
    schema JSONB NOT NULL,
    fetched_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
