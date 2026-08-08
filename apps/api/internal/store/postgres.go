package store

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/wreckr/wreckr/apps/api/internal/report"
	"github.com/wreckr/wreckr/apps/api/internal/runevent"
	"github.com/wreckr/wreckr/apps/api/internal/scenario"
)

const defaultProjectSlug = "default"

type Postgres struct {
	db        *sql.DB
	projectID string
}

var _ Store = (*Postgres)(nil)

func NewPostgres(ctx context.Context, databaseURL string) (*Postgres, error) {
	db, err := sql.Open("pgx", databaseURL)
	if err != nil {
		return nil, err
	}
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, err
	}

	p := &Postgres{db: db}
	if err := p.ensureDefaultProject(ctx); err != nil {
		_ = db.Close()
		return nil, err
	}
	return p, nil
}

func (p *Postgres) Close() error {
	return p.db.Close()
}

func (p *Postgres) CreateTarget(target TargetRecord) TargetRecord {
	ctx, cancel := storeContext()
	defer cancel()

	headers := target.Headers
	if headers == nil {
		headers = map[string]string{}
	}
	rawHeaders, err := json.Marshal(headers)
	if err != nil {
		return TargetRecord{}
	}

	slug := uniqueSlug(target.Name)
	var record TargetRecord
	var environment string
	err = p.db.QueryRowContext(ctx, `
		INSERT INTO targets (project_id, name, slug, protocol, base_url, environment, description, headers)
		VALUES ($1, $2, $3, 'http', $4, $5, $6, $7)
		RETURNING id::text, name, base_url, environment::text, description, headers, created_at, updated_at
	`, p.projectID, target.Name, slug, target.BaseURL, target.Environment, target.Description, rawHeaders).Scan(
		&record.ID,
		&record.Name,
		&record.BaseURL,
		&environment,
		&record.Description,
		&rawHeaders,
		&record.CreatedAt,
		&record.UpdatedAt,
	)
	if err != nil {
		return TargetRecord{}
	}
	record.Environment = TargetEnvironment(environment)
	_ = json.Unmarshal(rawHeaders, &record.Headers)
	return record
}

func (p *Postgres) UpdateTarget(id string, target TargetRecord) (TargetRecord, bool) {
	ctx, cancel := storeContext()
	defer cancel()

	headers := target.Headers
	if headers == nil {
		headers = map[string]string{}
	}
	rawHeaders, err := json.Marshal(headers)
	if err != nil {
		return TargetRecord{}, false
	}

	var record TargetRecord
	var environment string
	err = p.db.QueryRowContext(ctx, `
		UPDATE targets
		SET name = $2,
			base_url = $3,
			environment = $4,
			description = $5,
			headers = $6
		WHERE id = $1 AND project_id = $7
		RETURNING id::text, name, base_url, environment::text, description, headers, created_at, updated_at
	`, id, target.Name, target.BaseURL, target.Environment, target.Description, rawHeaders, p.projectID).Scan(
		&record.ID,
		&record.Name,
		&record.BaseURL,
		&environment,
		&record.Description,
		&rawHeaders,
		&record.CreatedAt,
		&record.UpdatedAt,
	)
	if err != nil {
		return TargetRecord{}, false
	}
	record.Environment = TargetEnvironment(environment)
	_ = json.Unmarshal(rawHeaders, &record.Headers)
	return record, true
}

func (p *Postgres) DeleteTarget(id string) bool {
	ctx, cancel := storeContext()
	defer cancel()
	result, err := p.db.ExecContext(ctx, `
		DELETE FROM targets
		WHERE id = $1 AND project_id = $2
	`, id, p.projectID)
	if err != nil {
		return false
	}
	rows, err := result.RowsAffected()
	return err == nil && rows > 0
}

func (p *Postgres) GetTarget(id string) (TargetRecord, bool) {
	ctx, cancel := storeContext()
	defer cancel()

	var record TargetRecord
	var rawHeaders []byte
	var environment string
	err := p.db.QueryRowContext(ctx, `
		SELECT id::text, name, base_url, environment::text, description, headers, created_at, updated_at
		FROM targets
		WHERE id = $1 AND project_id = $2
	`, id, p.projectID).Scan(
		&record.ID,
		&record.Name,
		&record.BaseURL,
		&environment,
		&record.Description,
		&rawHeaders,
		&record.CreatedAt,
		&record.UpdatedAt,
	)
	if err != nil {
		return TargetRecord{}, false
	}
	record.Environment = TargetEnvironment(environment)
	_ = json.Unmarshal(rawHeaders, &record.Headers)
	return record, true
}

func (p *Postgres) ListTargets() []TargetRecord {
	ctx, cancel := storeContext()
	defer cancel()

	rows, err := p.db.QueryContext(ctx, `
		SELECT id::text, name, base_url, environment::text, description, headers, created_at, updated_at
		FROM targets
		WHERE project_id = $1
		ORDER BY created_at DESC
	`, p.projectID)
	if err != nil {
		return nil
	}
	defer rows.Close()

	var records []TargetRecord
	for rows.Next() {
		var record TargetRecord
		var rawHeaders []byte
		var environment string
		if err := rows.Scan(&record.ID, &record.Name, &record.BaseURL, &environment, &record.Description, &rawHeaders, &record.CreatedAt, &record.UpdatedAt); err != nil {
			return nil
		}
		record.Environment = TargetEnvironment(environment)
		_ = json.Unmarshal(rawHeaders, &record.Headers)
		records = append(records, record)
	}
	return records
}

func (p *Postgres) CreateScenario(sc scenario.Scenario) ScenarioRecord {
	ctx, cancel := storeContext()
	defer cancel()

	raw, checksum, err := marshalScenario(sc)
	if err != nil {
		return ScenarioRecord{}
	}

	tx, err := p.db.BeginTx(ctx, nil)
	if err != nil {
		return ScenarioRecord{}
	}
	defer rollbackUnlessCommitted(tx)

	slug := uniqueSlug(sc.Name)
	var record ScenarioRecord
	err = tx.QueryRowContext(ctx, `
		INSERT INTO scenarios (project_id, name, slug, description)
		VALUES ($1, $2, $3, $4)
		RETURNING id::text, created_at
	`, p.projectID, sc.Name, slug, sc.Description).Scan(&record.ID, &record.CreatedAt)
	if err != nil {
		return ScenarioRecord{}
	}

	var versionID string
	err = tx.QueryRowContext(ctx, `
		INSERT INTO scenario_versions (scenario_id, version_number, status, definition, checksum)
		VALUES ($1, 1, 'active', $2, $3)
		RETURNING id::text
	`, record.ID, raw, checksum).Scan(&versionID)
	if err != nil {
		return ScenarioRecord{}
	}

	if _, err := tx.ExecContext(ctx, `
		UPDATE scenarios
		SET current_version_id = $1
		WHERE id = $2
	`, versionID, record.ID); err != nil {
		return ScenarioRecord{}
	}
	if err := tx.Commit(); err != nil {
		return ScenarioRecord{}
	}

	record.Scenario = sc
	record.CurrentVersionID = versionID
	record.CurrentVersionNumber = 1
	return record
}

func (p *Postgres) UpdateScenario(id string, sc scenario.Scenario) (ScenarioRecord, bool) {
	ctx, cancel := storeContext()
	defer cancel()

	raw, checksum, err := marshalScenario(sc)
	if err != nil {
		return ScenarioRecord{}, false
	}

	tx, err := p.db.BeginTx(ctx, nil)
	if err != nil {
		return ScenarioRecord{}, false
	}
	defer rollbackUnlessCommitted(tx)

	var versionNumber int
	err = tx.QueryRowContext(ctx, `
		SELECT COALESCE(MAX(version_number), 0) + 1
		FROM scenario_versions
		WHERE scenario_id = $1
	`, id).Scan(&versionNumber)
	if err != nil || versionNumber == 1 {
		return ScenarioRecord{}, false
	}

	var versionID string
	err = tx.QueryRowContext(ctx, `
		INSERT INTO scenario_versions (scenario_id, version_number, status, definition, checksum)
		VALUES ($1, $2, 'active', $3, $4)
		RETURNING id::text
	`, id, versionNumber, raw, checksum).Scan(&versionID)
	if err != nil {
		return ScenarioRecord{}, false
	}

	var record ScenarioRecord
	err = tx.QueryRowContext(ctx, `
		UPDATE scenarios
		SET name = $2,
			description = $3,
			current_version_id = $4
		WHERE id = $1
		RETURNING id::text, created_at
	`, id, sc.Name, sc.Description, versionID).Scan(&record.ID, &record.CreatedAt)
	if err != nil {
		return ScenarioRecord{}, false
	}
	if err := tx.Commit(); err != nil {
		return ScenarioRecord{}, false
	}

	record.Scenario = sc
	record.CurrentVersionID = versionID
	record.CurrentVersionNumber = versionNumber
	return record, true
}

func (p *Postgres) GetScenario(id string) (ScenarioRecord, bool) {
	ctx, cancel := storeContext()
	defer cancel()

	var record ScenarioRecord
	var raw []byte
	err := p.db.QueryRowContext(ctx, `
		SELECT s.id::text, sv.id::text, sv.version_number, sv.definition, s.created_at
		FROM scenarios s
		JOIN scenario_versions sv ON sv.id = s.current_version_id
		WHERE s.id = $1
	`, id).Scan(&record.ID, &record.CurrentVersionID, &record.CurrentVersionNumber, &raw, &record.CreatedAt)
	if err != nil {
		return ScenarioRecord{}, false
	}
	if err := json.Unmarshal(raw, &record.Scenario); err != nil {
		return ScenarioRecord{}, false
	}
	return record, true
}

func (p *Postgres) ListScenarios() []ScenarioRecord {
	ctx, cancel := storeContext()
	defer cancel()

	rows, err := p.db.QueryContext(ctx, `
		SELECT s.id::text, sv.id::text, sv.version_number, sv.definition, s.created_at
		FROM scenarios s
		JOIN scenario_versions sv ON sv.id = s.current_version_id
		WHERE s.project_id = $1
		ORDER BY s.created_at DESC
	`, p.projectID)
	if err != nil {
		return nil
	}
	defer rows.Close()

	var records []ScenarioRecord
	for rows.Next() {
		var record ScenarioRecord
		var raw []byte
		if err := rows.Scan(&record.ID, &record.CurrentVersionID, &record.CurrentVersionNumber, &raw, &record.CreatedAt); err != nil {
			return nil
		}
		if err := json.Unmarshal(raw, &record.Scenario); err != nil {
			return nil
		}
		records = append(records, record)
	}
	return records
}

func (p *Postgres) ListScenarioVersions(id string) []ScenarioVersionRecord {
	ctx, cancel := storeContext()
	defer cancel()

	rows, err := p.db.QueryContext(ctx, `
		SELECT id::text, scenario_id::text, version_number, definition, created_at
		FROM scenario_versions
		WHERE scenario_id = $1
		ORDER BY version_number ASC
	`, id)
	if err != nil {
		return nil
	}
	defer rows.Close()

	var records []ScenarioVersionRecord
	for rows.Next() {
		var record ScenarioVersionRecord
		var raw []byte
		if err := rows.Scan(&record.ID, &record.ScenarioID, &record.VersionNumber, &raw, &record.CreatedAt); err != nil {
			return nil
		}
		if err := json.Unmarshal(raw, &record.Scenario); err != nil {
			return nil
		}
		records = append(records, record)
	}
	return records
}

func (p *Postgres) GetScenarioVersion(id string, versionNumber int) (ScenarioVersionRecord, bool) {
	ctx, cancel := storeContext()
	defer cancel()

	var record ScenarioVersionRecord
	var raw []byte
	err := p.db.QueryRowContext(ctx, `
		SELECT id::text, scenario_id::text, version_number, definition, created_at
		FROM scenario_versions
		WHERE scenario_id = $1 AND version_number = $2
	`, id, versionNumber).Scan(&record.ID, &record.ScenarioID, &record.VersionNumber, &raw, &record.CreatedAt)
	if err != nil {
		return ScenarioVersionRecord{}, false
	}
	if err := json.Unmarshal(raw, &record.Scenario); err != nil {
		return ScenarioVersionRecord{}, false
	}
	return record, true
}

func (p *Postgres) CreateRun(
	scenarioID string,
	targetID string,
	sc scenario.Scenario,
	versionRef ...ScenarioVersionRecord,
) RunRecord {
	ctx, cancel := storeContext()
	defer cancel()

	raw, _, err := marshalScenario(sc)
	if err != nil {
		return RunRecord{}
	}

	var scenarioParam any
	var targetParam any
	var versionParam any
	var versionNumber int

	if targetID != "" {
		targetParam = targetID
	}

	if len(versionRef) > 0 {
		resolvedScenarioID := scenarioID
		if resolvedScenarioID == "" {
			resolvedScenarioID = versionRef[0].ScenarioID
		}

		if resolvedScenarioID != "" {
			scenarioParam = resolvedScenarioID
		}

		versionParam = versionRef[0].ID
		versionNumber = versionRef[0].VersionNumber
	} else if scenarioID != "" {
		versionID, currentVersionNumber, ok := p.currentScenarioVersion(ctx, scenarioID)
		if ok {
			scenarioParam = scenarioID
			versionParam = versionID
			versionNumber = currentVersionNumber
		}
	}

	var record RunRecord
	err = p.db.QueryRowContext(ctx, `
		INSERT INTO runs (project_id, target_id, scenario_id, scenario_version_id, status, scenario_snapshot)
		VALUES ($1, $2, $3, $4, 'queued', $5)
		RETURNING id::text,
			COALESCE(target_id::text, ''),
			COALESCE(scenario_id::text, ''),
			COALESCE(scenario_version_id::text, ''),
			status::text,
			created_at
	`, p.projectID, targetParam, scenarioParam, versionParam, raw).Scan(
		&record.ID,
		&record.TargetID,
		&record.ScenarioID,
		&record.ScenarioVersionID,
		&record.Status,
		&record.CreatedAt,
	)
	if err != nil {
		return RunRecord{}
	}

	record.Scenario = sc
	record.ScenarioVersionNumber = versionNumber

	p.AppendRunEvent(record.ID, runevent.Event{
		Level:   runevent.LevelInfo,
		Type:    runevent.TypeRunQueued,
		Message: "run queued",
		Metadata: map[string]any{
			"scenario_id":             record.ScenarioID,
			"scenario_version_id":     record.ScenarioVersionID,
			"scenario_version_number": record.ScenarioVersionNumber,
			"scenario":                sc.Name,
			"target_id":               record.TargetID,
		},
	})

	return record
}

func (p *Postgres) MarkRunStarted(id string) {
	ctx, cancel := storeContext()
	defer cancel()

	result, err := p.db.ExecContext(ctx, `
		UPDATE runs
		SET status = 'running', started_at = COALESCE(started_at, now())
		WHERE id = $1 AND status IN ('queued', 'running')
	`, id)
	if err != nil {
		return
	}
	rows, err := result.RowsAffected()
	if err != nil || rows == 0 {
		return
	}
	p.appendRunEvent(ctx, id, runevent.Event{
		Level:   runevent.LevelInfo,
		Type:    runevent.TypeRunStarted,
		Message: "run started",
	})
}

func (p *Postgres) CompleteRun(id string, rep report.Report) {
	status := RunPassed
	if rep.Status == report.StatusFailed {
		status = RunFailed
	}
	if !p.finishRunWithReport(id, status, rep, "") {
		return
	}
	eventType := runevent.TypeRunCompleted
	level := runevent.LevelInfo
	message := "run completed"
	if status == RunFailed {
		eventType = runevent.TypeRunFailed
		level = runevent.LevelError
		message = "run failed"
	}
	p.AppendRunEvent(id, runevent.Event{
		Level:   level,
		Type:    eventType,
		Message: message,
		Metadata: map[string]any{
			"status":          string(status),
			"total_requests":  rep.Summary.TotalRequests,
			"failed_requests": rep.Summary.FailedRequests,
			"failures":        rep.Failures,
		},
	})
}

func (p *Postgres) CancelRun(id string, rep report.Report) {
	rep.Status = report.StatusCanceled
	if !p.finishRunWithReport(id, RunCanceled, rep, "run canceled") {
		return
	}
	p.AppendRunEvent(id, runevent.Event{
		Level:   runevent.LevelWarn,
		Type:    runevent.TypeRunCanceled,
		Message: "run canceled",
		Metadata: map[string]any{
			"failures": rep.Failures,
		},
	})
}

func (p *Postgres) finishRunWithReport(id string, status RunStatus, rep report.Report, errorMessage string) bool {
	ctx, cancel := storeContext()
	defer cancel()

	run, ok := p.getRun(ctx, id)
	if ok {
		rep.ScenarioVersionID = run.ScenarioVersionID
		rep.ScenarioVersionNumber = run.ScenarioVersionNumber
	}

	summary, _ := json.Marshal(rep.Summary)
	thresholds, _ := json.Marshal(rep.Thresholds)
	invariants, _ := json.Marshal(rep.Invariants)
	failures, _ := json.Marshal(rep.Failures)

	tx, err := p.db.BeginTx(ctx, nil)
	if err != nil {
		return false
	}
	defer rollbackUnlessCommitted(tx)

	result, err := tx.ExecContext(ctx, `
		UPDATE runs
		SET status = $2, finished_at = now(), error = NULLIF($3, '')
		WHERE id = $1 AND status IN ('queued', 'running')
	`, id, status, errorMessage)
	if err != nil {
		return false
	}
	rows, err := result.RowsAffected()
	if err != nil || rows == 0 {
		return false
	}

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO reports (run_id, status, summary, thresholds, invariants, failures)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (run_id)
		DO UPDATE SET
			status = EXCLUDED.status,
			summary = EXCLUDED.summary,
			thresholds = EXCLUDED.thresholds,
			invariants = EXCLUDED.invariants,
			failures = EXCLUDED.failures
	`, id, status, summary, thresholds, invariants, failures); err != nil {
		return false
	}

	return tx.Commit() == nil
}

func (p *Postgres) ErrorRun(id string, err error) {
	ctx, cancel := storeContext()
	defer cancel()

	message := ""
	if err != nil {
		message = err.Error()
	}
	result, err := p.db.ExecContext(ctx, `
		UPDATE runs
		SET status = 'errored', error = $2, finished_at = now()
		WHERE id = $1 AND status IN ('queued', 'running')
	`, id, message)
	if err != nil {
		return
	}
	rows, err := result.RowsAffected()
	if err != nil || rows == 0 {
		return
	}
	p.appendRunEvent(ctx, id, runevent.Event{
		Level:   runevent.LevelError,
		Type:    runevent.TypeRunFailed,
		Message: message,
	})
}

func (p *Postgres) GetRun(id string) (RunRecord, bool) {
	ctx, cancel := storeContext()
	defer cancel()

	record, ok := p.getRun(ctx, id)
	if !ok {
		return RunRecord{}, false
	}
	rep, ok := p.getReport(ctx, id)
	if ok {
		record.Report = &rep
	}
	return record, true
}

func (p *Postgres) ListRuns() []RunRecord {
	ctx, cancel := storeContext()
	defer cancel()

	// ⚡ Bolt: Optimized ListRuns to eliminate an N+1 query bottleneck.
	// Previously, this function iterated over a list of IDs and called GetRun for each,
	// resulting in multiple database roundtrips per row. Now it uses a single query
	// with LEFT JOINs to fetch the run records and their associated reports efficiently.
	rows, err := p.db.QueryContext(ctx, `
		SELECT
			runs.id::text,
			COALESCE(runs.target_id::text, ''),
			COALESCE(runs.scenario_id::text, ''),
			COALESCE(runs.scenario_version_id::text, ''),
			COALESCE(sv.version_number, 0),
			runs.status::text,
			runs.scenario_snapshot,
			runs.error,
			runs.created_at,
			runs.started_at,
			runs.finished_at,
			rep.status::text,
			rep.summary,
			rep.thresholds,
			rep.invariants,
			rep.failures
		FROM runs
		LEFT JOIN scenario_versions sv ON sv.id = runs.scenario_version_id
		LEFT JOIN reports rep ON rep.run_id = runs.id
		WHERE runs.project_id = $1
		ORDER BY runs.created_at DESC
	`, p.projectID)
	if err != nil {
		return nil
	}
	defer rows.Close()

	var records []RunRecord
	for rows.Next() {
		var record RunRecord
		var raw []byte
		var startedAt sql.NullTime
		var finishedAt sql.NullTime
		var errorText sql.NullString

		var repStatus sql.NullString
		var repSummary []byte
		var repThresholds []byte
		var repInvariants []byte
		var repFailures []byte

		if err := rows.Scan(
			&record.ID,
			&record.TargetID,
			&record.ScenarioID,
			&record.ScenarioVersionID,
			&record.ScenarioVersionNumber,
			&record.Status,
			&raw,
			&errorText,
			&record.CreatedAt,
			&startedAt,
			&finishedAt,
			&repStatus,
			&repSummary,
			&repThresholds,
			&repInvariants,
			&repFailures,
		); err != nil {
			return nil
		}

		if err := json.Unmarshal(raw, &record.Scenario); err != nil {
			continue
		}
		if errorText.Valid {
			record.Error = errorText.String
		}
		if startedAt.Valid {
			record.StartedAt = &startedAt.Time
		}
		if finishedAt.Valid {
			record.FinishedAt = &finishedAt.Time
		}

		if repStatus.Valid {
			rep := report.Report{
				RunID:                 record.ID,
				Status:                report.Status(repStatus.String),
				Scenario:              record.Scenario.Name,
				ScenarioVersionID:     record.ScenarioVersionID,
				ScenarioVersionNumber: record.ScenarioVersionNumber,
			}
			if len(repSummary) > 0 {
				_ = json.Unmarshal(repSummary, &rep.Summary)
			}
			if len(repThresholds) > 0 {
				_ = json.Unmarshal(repThresholds, &rep.Thresholds)
			}
			if len(repInvariants) > 0 {
				_ = json.Unmarshal(repInvariants, &rep.Invariants)
			}
			if len(repFailures) > 0 {
				_ = json.Unmarshal(repFailures, &rep.Failures)
			}
			record.Report = &rep
		}

		records = append(records, record)
	}
	return records
}

func (p *Postgres) RequestRunCancel(id string) bool {
	ctx, cancel := storeContext()
	defer cancel()

	tx, err := p.db.BeginTx(ctx, nil)
	if err != nil {
		return false
	}
	defer rollbackUnlessCommitted(tx)

	var status RunStatus
	if err := tx.QueryRowContext(ctx, `
		SELECT status::text
		FROM runs
		WHERE id = $1
		FOR UPDATE
	`, id).Scan(&status); err != nil {
		return false
	}
	if !isCancelableRunStatus(status) {
		return false
	}

	var alreadyRequested bool
	if err := tx.QueryRowContext(ctx, `
		SELECT EXISTS (
			SELECT 1
			FROM run_events
			WHERE run_id = $1 AND type = $2
		)
	`, id, runevent.TypeCancelRequested).Scan(&alreadyRequested); err != nil {
		return false
	}
	if alreadyRequested {
		return tx.Commit() == nil
	}

	if _, err := tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtext($1))`, id); err != nil {
		return false
	}
	var sequence int64
	if err := tx.QueryRowContext(ctx, `
		SELECT COALESCE(MAX(sequence), 0) + 1
		FROM run_events
		WHERE run_id = $1
	`, id).Scan(&sequence); err != nil {
		return false
	}
	raw, err := json.Marshal(map[string]any{"status": string(status)})
	if err != nil {
		return false
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO run_events (run_id, sequence, level, type, message, data, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`, id, sequence, runevent.LevelWarn, runevent.TypeCancelRequested, "run cancellation requested", raw, time.Now().UTC()); err != nil {
		return false
	}
	return tx.Commit() == nil
}

func (p *Postgres) IsRunCancelRequested(id string) bool {
	ctx, cancel := storeContext()
	defer cancel()

	var requested bool
	err := p.db.QueryRowContext(ctx, `
		SELECT EXISTS (
			SELECT 1
			FROM run_events
			WHERE run_id = $1 AND type = $2
		)
	`, id, runevent.TypeCancelRequested).Scan(&requested)
	return err == nil && requested
}

func (p *Postgres) AppendRunEvent(runID string, event runevent.Event) runevent.Event {
	ctx, cancel := storeContext()
	defer cancel()
	return p.appendRunEvent(ctx, runID, event)
}

func (p *Postgres) ListRunEvents(runID string) []runevent.Event {
	ctx, cancel := storeContext()
	defer cancel()

	rows, err := p.db.QueryContext(ctx, `
		SELECT id::text, run_id::text, sequence, level, type, message, data, created_at
		FROM run_events
		WHERE run_id = $1
		ORDER BY sequence ASC, created_at ASC
	`, runID)
	if err != nil {
		return nil
	}
	defer rows.Close()

	var events []runevent.Event
	for rows.Next() {
		var event runevent.Event
		var raw []byte
		if err := rows.Scan(&event.ID, &event.RunID, &event.Sequence, &event.Level, &event.Type, &event.Message, &raw, &event.CreatedAt); err != nil {
			return nil
		}
		if len(raw) > 0 {
			_ = json.Unmarshal(raw, &event.Metadata)
		}
		events = append(events, event)
	}
	return events
}

func (p *Postgres) ensureDefaultProject(ctx context.Context) error {
	return p.db.QueryRowContext(ctx, `
		INSERT INTO projects (name, slug, description)
		VALUES ('Default Project', $1, 'Default Wreckr project')
		ON CONFLICT (slug)
		DO UPDATE SET name = EXCLUDED.name
		RETURNING id::text
	`, defaultProjectSlug).Scan(&p.projectID)
}

func (p *Postgres) currentScenarioVersion(ctx context.Context, scenarioID string) (string, int, bool) {
	var versionID string
	var versionNumber int
	err := p.db.QueryRowContext(ctx, `
		SELECT sv.id::text, sv.version_number
		FROM scenarios s
		JOIN scenario_versions sv ON sv.id = s.current_version_id
		WHERE s.id = $1
	`, scenarioID).Scan(&versionID, &versionNumber)
	return versionID, versionNumber, err == nil
}

func (p *Postgres) getRun(ctx context.Context, id string) (RunRecord, bool) {
	var record RunRecord
	var raw []byte
	var startedAt sql.NullTime
	var finishedAt sql.NullTime
	var errorText sql.NullString
	err := p.db.QueryRowContext(ctx, `
		SELECT runs.id::text,
			COALESCE(runs.target_id::text, ''),
			COALESCE(runs.scenario_id::text, ''),
			COALESCE(runs.scenario_version_id::text, ''),
			COALESCE(sv.version_number, 0),
			runs.status::text,
			runs.scenario_snapshot,
			runs.error,
			runs.created_at,
			runs.started_at,
			runs.finished_at
		FROM runs
		LEFT JOIN scenario_versions sv ON sv.id = runs.scenario_version_id
		WHERE runs.id = $1
	`, id).Scan(
		&record.ID,
		&record.TargetID,
		&record.ScenarioID,
		&record.ScenarioVersionID,
		&record.ScenarioVersionNumber,
		&record.Status,
		&raw,
		&errorText,
		&record.CreatedAt,
		&startedAt,
		&finishedAt,
	)
	if err != nil {
		return RunRecord{}, false
	}
	if err := json.Unmarshal(raw, &record.Scenario); err != nil {
		return RunRecord{}, false
	}
	if errorText.Valid {
		record.Error = errorText.String
	}
	if startedAt.Valid {
		record.StartedAt = &startedAt.Time
	}
	if finishedAt.Valid {
		record.FinishedAt = &finishedAt.Time
	}
	return record, true
}

func (p *Postgres) getReport(ctx context.Context, runID string) (report.Report, bool) {
	var (
		status     string
		summary    []byte
		thresholds []byte
		invariants []byte
		failures   []byte
	)
	err := p.db.QueryRowContext(ctx, `
		SELECT status::text, summary, thresholds, invariants, failures
		FROM reports
		WHERE run_id = $1
	`, runID).Scan(&status, &summary, &thresholds, &invariants, &failures)
	if err != nil {
		return report.Report{}, false
	}

	var rep report.Report
	rep.RunID = runID
	rep.Status = report.Status(status)

	var scSnapshot []byte
	var scenarioVersionID string
	var scenarioVersionNum int
	err = p.db.QueryRowContext(ctx, `
		SELECT scenario_snapshot, scenario_version_id::text, scenario_version_number
		FROM runs
		WHERE id = $1
	`, runID).Scan(&scSnapshot, &scenarioVersionID, &scenarioVersionNum)
	if err == nil {
		var sc struct {
			Name string `json:"name"`
		}
		if err := json.Unmarshal(scSnapshot, &sc); err == nil {
			rep.Scenario = sc.Name
		}
		rep.ScenarioVersionID = scenarioVersionID
		rep.ScenarioVersionNumber = scenarioVersionNum
	}

	_ = json.Unmarshal(summary, &rep.Summary)
	_ = json.Unmarshal(thresholds, &rep.Thresholds)
	_ = json.Unmarshal(invariants, &rep.Invariants)
	_ = json.Unmarshal(failures, &rep.Failures)

	return rep, true
}

func (p *Postgres) appendRunEvent(ctx context.Context, runID string, event runevent.Event) runevent.Event {
	if event.RunID == "" {
		event.RunID = runID
	}
	if event.Level == "" {
		event.Level = runevent.LevelInfo
	}
	if event.CreatedAt.IsZero() {
		event.CreatedAt = time.Now().UTC()
	}
	metadata := event.Metadata
	if metadata == nil {
		metadata = map[string]any{}
	}
	raw, err := json.Marshal(metadata)
	if err != nil {
		return event
	}

	tx, err := p.db.BeginTx(ctx, nil)
	if err != nil {
		return event
	}
	defer rollbackUnlessCommitted(tx)

	if _, err := tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtext($1))`, runID); err != nil {
		return event
	}
	var sequence int64
	if err := tx.QueryRowContext(ctx, `
		SELECT COALESCE(MAX(sequence), 0) + 1
		FROM run_events
		WHERE run_id = $1
	`, runID).Scan(&sequence); err != nil {
		return event
	}
	err = tx.QueryRowContext(ctx, `
		INSERT INTO run_events (run_id, sequence, level, type, message, data, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id::text, sequence, created_at
	`, runID, sequence, event.Level, event.Type, event.Message, raw, event.CreatedAt).Scan(&event.ID, &event.Sequence, &event.CreatedAt)
	if err != nil {
		return event
	}
	if err := tx.Commit(); err != nil {
		return event
	}
	event.Metadata = metadata
	return event
}

func marshalScenario(sc scenario.Scenario) ([]byte, string, error) {
	raw, err := json.Marshal(sc)
	if err != nil {
		return nil, "", err
	}
	sum := sha256.Sum256(raw)
	return raw, hex.EncodeToString(sum[:]), nil
}

func storeContext() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), 5*time.Second)
}

func rollbackUnlessCommitted(tx *sql.Tx) {
	if err := tx.Rollback(); err != nil && !errors.Is(err, sql.ErrTxDone) {
		return
	}
}

var nonSlugChars = regexp.MustCompile(`[^a-z0-9]+`)

func uniqueSlug(value string) string {
	base := strings.ToLower(strings.TrimSpace(value))
	base = nonSlugChars.ReplaceAllString(base, "-")
	base = strings.Trim(base, "-")
	if base == "" {
		base = "scenario"
	}
	return fmt.Sprintf("%s-%d", base, time.Now().UTC().UnixNano())
}
