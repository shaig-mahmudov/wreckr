package worker

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/wreckr/wreckr/apps/api/internal/report"
	"github.com/wreckr/wreckr/apps/api/internal/runexec"
	"github.com/wreckr/wreckr/apps/api/internal/runner"
	"github.com/wreckr/wreckr/apps/api/internal/runqueue"
	"github.com/wreckr/wreckr/apps/api/internal/scenario"
	"github.com/wreckr/wreckr/apps/api/internal/store"
)

func TestHandlerExecutesQueuedRunTask(t *testing.T) {
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/work" {
			http.NotFound(w, r)
			return
		}
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{"ok": true})
	}))
	defer target.Close()

	st := store.NewMemory()
	run := st.CreateRun("", "", scenario.Scenario{
		Version: 1,
		Name:    "worker-test",
		Target:  scenario.Target{BaseURL: target.URL},
		Traffic: scenario.Traffic{
			Type:        scenario.TrafficLoad,
			Concurrency: 1,
			Iterations:  1,
		},
		Requests: []scenario.Request{{
			Name:   "work",
			Method: http.MethodGet,
			Path:   "/work",
			Expect: scenario.RequestExpectation{Status: []int{http.StatusOK}},
		}},
	})
	task, err := runqueue.NewRunTask(run.ID)
	if err != nil {
		t.Fatal(err)
	}

	handler := Handler{Executor: runexec.Executor{
		Store:   st,
		Runner:  runner.New(),
		Timeout: time.Second,
	}}
	if err := handler.handleRunExecute(context.Background(), task); err != nil {
		t.Fatal(err)
	}

	completed, ok := st.GetRun(run.ID)
	if !ok {
		t.Fatal("completed run was not found")
	}
	if completed.Status != store.RunPassed {
		t.Fatalf("run status = %s, want %s", completed.Status, store.RunPassed)
	}
	if completed.Report == nil {
		t.Fatal("completed run missing report")
	}
	if completed.Report.Status != report.StatusPassed {
		t.Fatalf("report status = %s, want %s", completed.Report.Status, report.StatusPassed)
	}
}
