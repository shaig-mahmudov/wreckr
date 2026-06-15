CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE OR REPLACE FUNCTION set_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = now();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TYPE target_protocol AS ENUM (
    'http',
    'grpc',
    'websocket',
    'queue'
);

CREATE TYPE target_environment AS ENUM (
    'local',
    'development',
    'staging',
    'production'
);

CREATE TYPE scenario_version_status AS ENUM (
    'draft',
    'active',
    'archived'
);

CREATE TYPE run_status AS ENUM (
    'queued',
    'running',
    'passed',
    'failed',
    'errored',
    'canceled'
);

CREATE TABLE projects (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name TEXT NOT NULL,
    slug TEXT NOT NULL UNIQUE,
    description TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE targets (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    slug TEXT NOT NULL,
    protocol target_protocol NOT NULL DEFAULT 'http',
    base_url TEXT NOT NULL,
    environment target_environment NOT NULL DEFAULT 'development',
    description TEXT NOT NULL DEFAULT '',
    headers JSONB NOT NULL DEFAULT '{}'::jsonb,
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (project_id, slug)
);

CREATE TABLE scenarios (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    slug TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    current_version_id UUID,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (project_id, slug)
);

CREATE TABLE scenario_versions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    scenario_id UUID NOT NULL REFERENCES scenarios(id) ON DELETE CASCADE,
    version_number INTEGER NOT NULL,
    status scenario_version_status NOT NULL DEFAULT 'draft',
    definition JSONB NOT NULL,
    checksum TEXT NOT NULL,
    created_by TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (scenario_id, version_number)
);

ALTER TABLE scenarios
    ADD CONSTRAINT scenarios_current_version_id_fkey
    FOREIGN KEY (current_version_id)
    REFERENCES scenario_versions(id)
    ON DELETE SET NULL;

CREATE TABLE runs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    target_id UUID REFERENCES targets(id) ON DELETE SET NULL,
    scenario_id UUID REFERENCES scenarios(id) ON DELETE SET NULL,
    scenario_version_id UUID REFERENCES scenario_versions(id) ON DELETE SET NULL,
    status run_status NOT NULL DEFAULT 'queued',
    scenario_snapshot JSONB NOT NULL,
    requested_by TEXT,
    error TEXT,
    queued_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    started_at TIMESTAMPTZ,
    finished_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK (finished_at IS NULL OR started_at IS NULL OR finished_at >= started_at)
);

CREATE TABLE run_events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    run_id UUID NOT NULL REFERENCES runs(id) ON DELETE CASCADE,
    sequence BIGINT NOT NULL,
    level TEXT NOT NULL DEFAULT 'info',
    type TEXT NOT NULL,
    message TEXT NOT NULL DEFAULT '',
    data JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (run_id, sequence)
);

CREATE TABLE reports (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    run_id UUID NOT NULL UNIQUE REFERENCES runs(id) ON DELETE CASCADE,
    status run_status NOT NULL,
    summary JSONB NOT NULL DEFAULT '{}'::jsonb,
    thresholds JSONB NOT NULL DEFAULT '[]'::jsonb,
    invariants JSONB NOT NULL DEFAULT '[]'::jsonb,
    failures JSONB NOT NULL DEFAULT '[]'::jsonb,
    raw_report JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK (status IN ('passed', 'failed', 'errored', 'canceled'))
);

CREATE INDEX targets_project_id_idx ON targets(project_id);
CREATE INDEX scenarios_project_id_idx ON scenarios(project_id);
CREATE INDEX scenario_versions_scenario_id_idx ON scenario_versions(scenario_id);
CREATE INDEX runs_project_id_status_idx ON runs(project_id, status);
CREATE INDEX runs_scenario_version_id_idx ON runs(scenario_version_id);
CREATE INDEX runs_target_id_idx ON runs(target_id);
CREATE INDEX run_events_run_id_sequence_idx ON run_events(run_id, sequence);
CREATE INDEX reports_run_id_idx ON reports(run_id);

CREATE TRIGGER projects_set_updated_at
    BEFORE UPDATE ON projects
    FOR EACH ROW
    EXECUTE FUNCTION set_updated_at();

CREATE TRIGGER targets_set_updated_at
    BEFORE UPDATE ON targets
    FOR EACH ROW
    EXECUTE FUNCTION set_updated_at();

CREATE TRIGGER scenarios_set_updated_at
    BEFORE UPDATE ON scenarios
    FOR EACH ROW
    EXECUTE FUNCTION set_updated_at();

CREATE TRIGGER scenario_versions_set_updated_at
    BEFORE UPDATE ON scenario_versions
    FOR EACH ROW
    EXECUTE FUNCTION set_updated_at();

CREATE TRIGGER runs_set_updated_at
    BEFORE UPDATE ON runs
    FOR EACH ROW
    EXECUTE FUNCTION set_updated_at();

CREATE TRIGGER run_events_set_updated_at
    BEFORE UPDATE ON run_events
    FOR EACH ROW
    EXECUTE FUNCTION set_updated_at();

CREATE TRIGGER reports_set_updated_at
    BEFORE UPDATE ON reports
    FOR EACH ROW
    EXECUTE FUNCTION set_updated_at();
