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
	return record
}

func (p *Postgres) GetScenario(id string) (ScenarioRecord, bool) {
	ctx, cancel := storeContext()
	defer cancel()

	var record ScenarioRecord
	var raw []byte
	err := p.db.QueryRowContext(ctx, `
		SELECT s.id::text, sv.definition, s.created_at
		FROM scenarios s
		JOIN scenario_versions sv ON sv.id = s.current_version_id
		WHERE s.id = $1
	`, id).Scan(&record.ID, &raw, &record.CreatedAt)
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
		SELECT s.id::text, sv.definition, s.created_at
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
		if err := rows.Scan(&record.ID, &raw, &record.CreatedAt); err != nil {
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
	if scenarioID != "" {
		versionID, ok := p.currentScenarioVersion(ctx, scenarioID)
		if ok {
			scenarioParam = scenarioID
			versionParam = versionID
		}
	}

	var record RunRecord
	err = p.db.QueryRowContext(ctx, `
		INSERT INTO runs (project_id, scenario_id, scenario_version_id, status, scenario_snapshot)
		VALUES ($1, $2, $3, 'queued', $4)
		RETURNING id::text, COALESCE(scenario_id::text, ''), status::text, created_at
	`, p.projectID, scenarioParam, versionParam, raw).Scan(&record.ID, &record.ScenarioID, &record.Status, &record.CreatedAt)
	if err != nil {
		return RunRecord{}
	}
	record.Scenario = sc
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
}

func (p *Postgres) CompleteRun(id string, rep report.Report) {
	ctx, cancel := storeContext()
	defer cancel()

	status := RunPassed
	if rep.Status == report.StatusFailed {
		status = RunFailed
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
		SET status = $2, finished_at = now(), error = NULL
		WHERE id = $1
	`, id, status); err != nil {
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

func (p *Postgres) ensureDefaultProject(ctx context.Context) error {
	return p.db.QueryRowContext(ctx, `
		INSERT INTO projects (name, slug, description)
		VALUES ('Default Project', $1, 'Default Wreckr project')
		ON CONFLICT (slug)
		DO UPDATE SET name = EXCLUDED.name
		RETURNING id::text
	`, defaultProjectSlug).Scan(&p.projectID)
}

func (p *Postgres) currentScenarioVersion(ctx context.Context, scenarioID string) (string, bool) {
	var versionID string
	err := p.db.QueryRowContext(ctx, `
		SELECT current_version_id::text
		FROM scenarios
		WHERE id = $1
	`, scenarioID).Scan(&versionID)
	return versionID, err == nil
}

func (p *Postgres) getRun(ctx context.Context, id string) (RunRecord, bool) {
	var record RunRecord
	var raw []byte
	var startedAt sql.NullTime
	var finishedAt sql.NullTime
	var errorText sql.NullString
	err := p.db.QueryRowContext(ctx, `
		SELECT id::text,
			COALESCE(scenario_id::text, ''),
			status::text,
			scenario_snapshot,
			error,
			created_at,
			started_at,
			finished_at
		FROM runs
		WHERE id = $1
	`, id).Scan(
		&record.ID,
		&record.ScenarioID,
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
