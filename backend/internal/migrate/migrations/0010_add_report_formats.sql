-- A report is delivered as one or more files, and which formats those are is
-- the client's choice per report. Reports created before this default to PDF,
-- which is what a report was implicitly going to be.
ALTER TABLE reports
    ADD COLUMN IF NOT EXISTS formats TEXT[] NOT NULL DEFAULT ARRAY['pdf'];

-- A report with no format would render into nothing, so the column can't be
-- emptied. cardinality reports 0 for an empty array, where array_length reports
-- NULL — and a check that evaluates to NULL passes. The format names themselves
-- are checked by the server, which is the only thing that knows which renderers
-- exist.
ALTER TABLE reports
    ADD CONSTRAINT reports_formats_not_empty CHECK (cardinality(formats) >= 1);
