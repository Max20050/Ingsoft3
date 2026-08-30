-- A report's query is now a specification the server compiles, not text a
-- client sends. The query column stays as the compiled SQL, kept so the user
-- can read what their report runs.
--
-- Reports created before this keep their old query text and an empty
-- specification, which marks them as needing to be rebuilt: nothing runs a
-- stored query as text any more.
ALTER TABLE reports
    ADD COLUMN IF NOT EXISTS query_spec JSONB NOT NULL DEFAULT '{}'::jsonb;

-- A report's description is no longer what builds its query, so it's optional.
ALTER TABLE reports
    ALTER COLUMN prompt SET DEFAULT '';
