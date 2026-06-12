package runexec

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/wreckr/wreckr/apps/api/internal/runner"
	"github.com/wreckr/wreckr/apps/api/internal/scenario"
	"github.com/wreckr/wreckr/apps/api/internal/store"
)

func TestExecutorCanResumeRunningRunOnRetry(t *testing.T) {
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer target.Close()

	st := store.NewMemory()
	run := st.CreateRun("", scenario.Scenario{
		Version: 1,
		Name:    "retry-resume",
		Target:  scenario.Target{BaseURL: target.URL},
		Traffic: scenario.Traffic{
			Type:        scenario.TrafficLoad,
			Concurrency: 1,
			Iterations:  1,
		},
		Requests: []scenario.Request{{
			Name:   "request",
			Method: http.MethodGet,
			Path:   "/",
			Expect: scenario.RequestExpectation{Status: []int{http.StatusOK}},
		}},
	})
	st.MarkRunStarted(run.ID)

	err := Executor{
		Store:   st,
		Runner:  runner.New(),
		Timeout: time.Second,
	}.Execute(context.Background(), run.ID)
	if err != nil {
		t.Fatal(err)
	}

	completed, ok := st.GetRun(run.ID)
	if !ok {
		t.Fatal("run was not found")
	}
	if completed.Status != store.RunPassed {
		t.Fatalf("run status = %s, want %s", completed.Status, store.RunPassed)
	}
}
