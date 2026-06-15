package store

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/wreckr/wreckr/apps/api/internal/report"
	"github.com/wreckr/wreckr/apps/api/internal/runevent"
	"github.com/wreckr/wreckr/apps/api/internal/scenario"
)

func TestPostgresStorePersistsScenariosRunsAndReports(t *testing.T) {
	databaseURL := os.Getenv("WRECKR_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("set WRECKR_TEST_DATABASE_URL to run PostgreSQL store integration tests")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	db, err := sql.Open("pgx", databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	resetPostgresSchema(t, ctx, db)

	st, err := NewPostgres(ctx, databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	target := st.CreateTarget(TargetRecord{
		Name:        "Postgres Demo Target",
		BaseURL:     "http://demo-api.test",
		Environment: TargetDevelopment,
		Description: "target persisted by postgres store test",
		Headers: map[string]string{
			"X-Wreckr-Target": "postgres-demo",
		},
	})
	if target.ID == "" {
		t.Fatal("created target ID is empty")
	}
	if target.BaseURL != "http://demo-api.test" {
		t.Fatalf("created target base URL = %q, want http://demo-api.test", target.BaseURL)
	}
	if target.Environment != TargetDevelopment {
		t.Fatalf("created target environment = %q, want %q", target.Environment, TargetDevelopment)
	}
	if target.Headers["X-Wreckr-Target"] != "postgres-demo" {
		t.Fatalf("created target headers = %#v, want X-Wreckr-Target", target.Headers)
	}

	gotTarget, ok := st.GetTarget(target.ID)
	if !ok {
		t.Fatal("created target was not found")
	}
	if gotTarget.Name != target.Name {
		t.Fatalf("target name = %q, want %q", gotTarget.Name, target.Name)
	}
	if listedTargets := st.ListTargets(); len(listedTargets) != 1 {
		t.Fatalf("target count = %d, want 1", len(listedTargets))
	}

	updatedTarget, ok := st.UpdateTarget(target.ID, TargetRecord{
		Name:        "Postgres Staging Target",
		BaseURL:     "http://staging-api.test",
		Environment: TargetStaging,
		Description: "updated target",
		Headers: map[string]string{
			"X-Wreckr-Target": "postgres-staging",
		},
	})
	if !ok {
		t.Fatal("target update failed")
	}
	if updatedTarget.BaseURL != "http://staging-api.test" {
		t.Fatalf("updated target base URL = %q, want http://staging-api.test", updatedTarget.BaseURL)
	}
	if updatedTarget.Environment != TargetStaging {
		t.Fatalf("updated target environment = %q, want %q", updatedTarget.Environment, TargetStaging)
	}

	sc := postgresTestScenario()
	created := st.CreateScenario(sc)
	if created.ID == "" {
		t.Fatal("created scenario ID is empty")
	}
	if created.CurrentVersionNumber != 1 {
		t.Fatalf("created scenario version = %d, want 1", created.CurrentVersionNumber)
	}

	gotScenario, ok := st.GetScenario(created.ID)
	if !ok {
		t.Fatal("created scenario was not found")
	}
	if gotScenario.Scenario.Name != sc.Name {
		t.Fatalf("scenario name = %q, want %q", gotScenario.Scenario.Name, sc.Name)
	}
	if listed := st.ListScenarios(); len(listed) != 1 {
		t.Fatalf("scenario count = %d, want 1", len(listed))
	}

	targetRunScenario := postgresTestScenario()
	targetRunScenario.Target.ID = updatedTarget.ID
	targetRunScenario.Target.BaseURL = updatedTarget.BaseURL
	targetRun := st.CreateRun(created.ID, updatedTarget.ID, targetRunScenario)
	if targetRun.ID == "" {
		t.Fatal("target-linked run ID is empty")
	}
	if targetRun.TargetID != updatedTarget.ID {
		t.Fatalf("target-linked run target ID = %q, want %q", targetRun.TargetID, updatedTarget.ID)
	}
	if targetRun.Scenario.Target.BaseURL != updatedTarget.BaseURL {
		t.Fatalf("target-linked run snapshot base URL = %q, want %q", targetRun.Scenario.Target.BaseURL, updatedTarget.BaseURL)
	}
	targetRunEvents := st.ListRunEvents(targetRun.ID)
	if len(targetRunEvents) != 1 || targetRunEvents[0].Metadata["target_id"] != updatedTarget.ID {
		t.Fatalf("target-linked run events = %#v, want queued event with target_id %q", targetRunEvents, updatedTarget.ID)
	}

	if !st.DeleteTarget(updatedTarget.ID) {
		t.Fatal("target delete failed")
	}
	if _, ok := st.GetTarget(updatedTarget.ID); ok {
		t.Fatal("deleted target was found")
	}
	targetRunAfterDelete, ok := st.GetRun(targetRun.ID)
	if !ok {
		t.Fatal("target-linked run was not found after target delete")
	}
	if targetRunAfterDelete.TargetID != "" {
		t.Fatalf("target-linked run target ID after delete = %q, want empty", targetRunAfterDelete.TargetID)
	}
	if targetRunAfterDelete.Scenario.Target.BaseURL != updatedTarget.BaseURL {
		t.Fatalf("target-linked run snapshot base URL after delete = %q, want %q", targetRunAfterDelete.Scenario.Target.BaseURL, updatedTarget.BaseURL)
	}

	run := st.CreateRun(created.ID, "", sc)
	if run.ID == "" {
		t.Fatal("created run ID is empty")
	}
	if run.Status != RunQueued {
		t.Fatalf("initial run status = %s, want %s", run.Status, RunQueued)
	}
	if run.ScenarioVersionNumber != 1 {
		t.Fatalf("run scenario version = %d, want 1", run.ScenarioVersionNumber)
	}
	events := st.ListRunEvents(run.ID)
	if len(events) != 1 || events[0].Type != runevent.TypeRunQueued {
		t.Fatalf("initial run events = %#v, want run_queued", events)
	}

	updatedScenario := postgresTestScenario()
	updatedScenario.Name = "postgres-store-test-v2"
	updated, ok := st.UpdateScenario(created.ID, updatedScenario)
	if !ok {
		t.Fatal("scenario update failed")
	}
	if updated.CurrentVersionNumber != 2 {
		t.Fatalf("updated scenario version = %d, want 2", updated.CurrentVersionNumber)
	}
	if versions := st.ListScenarioVersions(created.ID); len(versions) != 2 {
		t.Fatalf("scenario version count = %d, want 2", len(versions))
	}

	st.MarkRunStarted(run.ID)
	started, ok := st.GetRun(run.ID)
	if !ok {
		t.Fatal("started run was not found")
	}
	if started.Status != RunRunning {
		t.Fatalf("started run status = %s, want %s", started.Status, RunRunning)
	}
	if started.StartedAt == nil {
		t.Fatal("started run missing StartedAt")
	}
	events = st.ListRunEvents(run.ID)
	if len(events) < 2 || events[1].Type != runevent.TypeRunStarted {
		t.Fatalf("started run events = %#v, want second event run_started", events)
	}

	rep := report.Build(run.ID, sc.Name, time.Now().Add(-time.Second), []report.ResponseRecord{{
		RequestName: "request",
		StatusCode:  200,
		DurationMS:  12,
		StartedAt:   time.Now().Add(-time.Second),
	}}, nil, nil)
	st.CompleteRun(run.ID, rep)

	completed, ok := st.GetRun(run.ID)
	if !ok {
		t.Fatal("completed run was not found")
	}
	if completed.Status != RunPassed {
		t.Fatalf("completed run status = %s, want %s", completed.Status, RunPassed)
	}
	if completed.FinishedAt == nil {
		t.Fatal("completed run missing FinishedAt")
	}
	if completed.Report == nil {
		t.Fatal("completed run missing report")
	}
	if completed.ScenarioVersionNumber != 1 {
		t.Fatalf("completed run scenario version = %d, want 1", completed.ScenarioVersionNumber)
	}
	if completed.Report.ScenarioVersionNumber != 1 {
		t.Fatalf("completed report scenario version = %d, want 1", completed.Report.ScenarioVersionNumber)
	}
	if completed.Report.Scenario != "postgres-store-test" {
		t.Fatalf("completed report scenario = %q, want original scenario name", completed.Report.Scenario)
	}
	if completed.Report.Summary.TotalRequests != 1 {
		t.Fatalf("report total requests = %d, want 1", completed.Report.Summary.TotalRequests)
	}
	if listed := st.ListRuns(); len(listed) != 2 {
		t.Fatalf("run count = %d, want 2", len(listed))
	}
	events = st.ListRunEvents(run.ID)
	if len(events) < 3 || events[len(events)-1].Type != runevent.TypeRunCompleted {
		t.Fatalf("completed run events = %#v, want final event run_completed", events)
	}
	for i, event := range events {
		if event.Sequence != int64(i+1) {
			t.Fatalf("event %d sequence = %d, want %d", i, event.Sequence, i+1)
		}
	}

	cancelRun := st.CreateRun(created.ID, "", sc)
	st.MarkRunStarted(cancelRun.ID)
	cancelReport := report.Build(cancelRun.ID, sc.Name, time.Now().Add(-time.Second), []report.ResponseRecord{{
		RequestName: "request",
		Error:       "context canceled",
		DurationMS:  7,
		StartedAt:   time.Now().Add(-time.Second),
	}}, nil, nil)
	st.CancelRun(cancelRun.ID, cancelReport)

	canceled, ok := st.GetRun(cancelRun.ID)
	if !ok {
		t.Fatal("canceled run was not found")
	}
	if canceled.Status != RunCanceled {
		t.Fatalf("canceled run status = %s, want %s", canceled.Status, RunCanceled)
	}
	if canceled.Report == nil {
		t.Fatal("canceled run missing report")
	}
	if canceled.Report.Status != report.StatusCanceled {
		t.Fatalf("canceled report status = %s, want %s", canceled.Report.Status, report.StatusCanceled)
	}
	if canceled.FinishedAt == nil {
		t.Fatal("canceled run missing FinishedAt")
	}
}

func resetPostgresSchema(t *testing.T, ctx context.Context, db *sql.DB) {
	t.Helper()

	root := filepath.Join("..", "..", "..", "..")
	down, err := os.ReadFile(filepath.Join(root, "deployments", "postgres", "migrations", "000001_initial_schema.down.sql"))
	if err != nil {
		t.Fatal(err)
	}
	up, err := os.ReadFile(filepath.Join(root, "deployments", "postgres", "migrations", "000001_initial_schema.up.sql"))
	if err != nil {
		t.Fatal(err)
	}
	_, _ = db.ExecContext(ctx, string(down))
	if _, err := db.ExecContext(ctx, string(up)); err != nil {
		t.Fatal(err)
	}
}

func postgresTestScenario() scenario.Scenario {
	return scenario.Scenario{
		Version: 1,
		Name:    "postgres-store-test",
		Target:  scenario.Target{BaseURL: "http://example.test"},
		Traffic: scenario.Traffic{
			Type:        scenario.TrafficLoad,
			Concurrency: 1,
			Iterations:  1,
		},
		Requests: []scenario.Request{{
			Name:   "request",
			Method: "GET",
			Path:   "/healthz",
		}},
	}
}
