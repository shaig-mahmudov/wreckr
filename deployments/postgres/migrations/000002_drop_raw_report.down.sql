ALTER TABLE reports ADD COLUMN raw_report JSONB NOT NULL DEFAULT '{}'::jsonb;
