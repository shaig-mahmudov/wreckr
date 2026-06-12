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

func (p *Postgres) CreateRun(scenarioID string, sc scenario.Scenario) RunRecord {
	ctx, cancel := storeContext()
	defer cancel()

	raw, _, err := marshalScenario(sc)
	if err != nil {
		return RunRecord{}
	}

	var scenarioParam any
	var versionParam any
	var versionNumber int
	if scenarioID != "" {
		versionID, currentVersionNumber, ok := p.currentScenarioVersion(ctx, scenarioID)
		if ok {
			scenarioParam = scenarioID
			versionParam = versionID
			versionNumber = currentVersionNumber
		}
	}

	var record RunRecord
	err = p.db.QueryRowContext(ctx, `
		INSERT INTO runs (project_id, scenario_id, scenario_version_id, status, scenario_snapshot)
		VALUES ($1, $2, $3, 'queued', $4)
		RETURNING id::text, COALESCE(scenario_id::text, ''), COALESCE(scenario_version_id::text, ''), status::text, created_at
	`, p.projectID, scenarioParam, versionParam, raw).Scan(&record.ID, &record.ScenarioID, &record.ScenarioVersionID, &record.Status, &record.CreatedAt)
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
		},
	})
	return record
}

func (p *Postgres) MarkRunStarted(id string) {
	ctx, cancel := storeContext()
	defer cancel()

	_, _ = p.db.ExecContext(ctx, `
		UPDATE runs
		SET status = 'running', started_at = COALESCE(started_at, now())
		WHERE id = $1
	`, id)
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
	p.finishRunWithReport(id, status, rep, "")
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
	p.finishRunWithReport(id, RunCanceled, rep, "run canceled")
	p.AppendRunEvent(id, runevent.Event{
		Level:   runevent.LevelWarn,
		Type:    runevent.TypeRunCanceled,
		Message: "run canceled",
		Metadata: map[string]any{
			"failures": rep.Failures,
		},
	})
}

func (p *Postgres) finishRunWithReport(id string, status RunStatus, rep report.Report, errorMessage string) {
	ctx, cancel := storeContext()
	defer cancel()

	run, ok := p.getRun(ctx, id)
	if ok {
		rep.ScenarioVersionID = run.ScenarioVersionID
		rep.ScenarioVersionNumber = run.ScenarioVersionNumber
	}
	rawReport, err := json.Marshal(rep)
	if err != nil {
		return
	}
	summary, _ := json.Marshal(rep.Summary)
	thresholds, _ := json.Marshal(rep.Thresholds)
	invariants, _ := json.Marshal(rep.Invariants)
	failures, _ := json.Marshal(rep.Failures)

	tx, err := p.db.BeginTx(ctx, nil)
	if err != nil {
		return
	}
	defer rollbackUnlessCommitted(tx)

	if _, err := tx.ExecContext(ctx, `
		UPDATE runs
		SET status = $2, finished_at = now(), error = NULLIF($3, '')
		WHERE id = $1
	`, id, status, errorMessage); err != nil {
		return
	}

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO reports (run_id, status, summary, thresholds, invariants, failures, raw_report)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (run_id)
		DO UPDATE SET
			status = EXCLUDED.status,
			summary = EXCLUDED.summary,
			thresholds = EXCLUDED.thresholds,
			invariants = EXCLUDED.invariants,
			failures = EXCLUDED.failures,
			raw_report = EXCLUDED.raw_report
	`, id, status, summary, thresholds, invariants, failures, rawReport); err != nil {
		return
	}

	_ = tx.Commit()
}

func (p *Postgres) ErrorRun(id string, err error) {
	ctx, cancel := storeContext()
	defer cancel()

	message := ""
	if err != nil {
		message = err.Error()
	}
	_, _ = p.db.ExecContext(ctx, `
		UPDATE runs
		SET status = 'errored', error = $2, finished_at = now()
		WHERE id = $1
	`, id, message)
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

	rows, err := p.db.QueryContext(ctx, `
		SELECT id::text
		FROM runs
		WHERE project_id = $1
		ORDER BY created_at DESC
	`, p.projectID)
	if err != nil {
		return nil
	}
	defer rows.Close()

	var records []RunRecord
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil
		}
		record, ok := p.GetRun(id)
		if !ok {
			return nil
		}
		records = append(records, record)
	}
	return records
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
	var raw []byte
	err := p.db.QueryRowContext(ctx, `
		SELECT raw_report
		FROM reports
		WHERE run_id = $1
	`, runID).Scan(&raw)
	if err != nil {
		return report.Report{}, false
	}
	var rep report.Report
	if err := json.Unmarshal(raw, &rep); err != nil {
		return report.Report{}, false
	}
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
