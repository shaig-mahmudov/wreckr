DROP TRIGGER IF EXISTS reports_set_updated_at ON reports;
DROP TRIGGER IF EXISTS run_events_set_updated_at ON run_events;
DROP TRIGGER IF EXISTS runs_set_updated_at ON runs;
DROP TRIGGER IF EXISTS scenario_versions_set_updated_at ON scenario_versions;
DROP TRIGGER IF EXISTS scenarios_set_updated_at ON scenarios;
DROP TRIGGER IF EXISTS targets_set_updated_at ON targets;
DROP TRIGGER IF EXISTS projects_set_updated_at ON projects;

DROP INDEX IF EXISTS reports_run_id_idx;
DROP INDEX IF EXISTS run_events_run_id_sequence_idx;
DROP INDEX IF EXISTS runs_target_id_idx;
DROP INDEX IF EXISTS runs_scenario_version_id_idx;
DROP INDEX IF EXISTS runs_project_id_status_idx;
DROP INDEX IF EXISTS scenario_versions_scenario_id_idx;
DROP INDEX IF EXISTS scenarios_project_id_idx;
DROP INDEX IF EXISTS targets_project_id_idx;

DROP TABLE IF EXISTS reports;
DROP TABLE IF EXISTS run_events;
DROP TABLE IF EXISTS runs;

ALTER TABLE IF EXISTS scenarios
    DROP CONSTRAINT IF EXISTS scenarios_current_version_id_fkey;

DROP TABLE IF EXISTS scenario_versions;
DROP TABLE IF EXISTS scenarios;
DROP TABLE IF EXISTS targets;
DROP TABLE IF EXISTS projects;

DROP TYPE IF EXISTS run_status;
DROP TYPE IF EXISTS scenario_version_status;
DROP TYPE IF EXISTS target_protocol;

DROP FUNCTION IF EXISTS set_updated_at();
