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

	sc := postgresTestScenario()
	created := st.CreateScenario(sc)
	if created.ID == "" {
		t.Fatal("created scenario ID is empty")
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

	run := st.CreateRun(created.ID, sc)
	if run.ID == "" {
		t.Fatal("created run ID is empty")
	}
	if run.Status != RunQueued {
		t.Fatalf("initial run status = %s, want %s", run.Status, RunQueued)
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
	if completed.Report.Summary.TotalRequests != 1 {
		t.Fatalf("report total requests = %d, want 1", completed.Report.Summary.TotalRequests)
	}
	if listed := st.ListRuns(); len(listed) != 1 {
		t.Fatalf("run count = %d, want 1", len(listed))
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
