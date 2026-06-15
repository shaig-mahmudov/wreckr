package runexec

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/wreckr/wreckr/apps/api/internal/report"
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
	run := st.CreateRun("", "", scenario.Scenario{
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

func TestExecutorCancelsRunRequestedBeforeExecution(t *testing.T) {
	var requests int
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests++
		w.WriteHeader(http.StatusOK)
	}))
	defer target.Close()

	st := store.NewMemory()
	run := st.CreateRun("", "", testRunScenario("cancel-before-execution", target.URL, 1))
	if !st.RequestRunCancel(run.ID) {
		t.Fatal("cancel request was not recorded")
	}

	err := Executor{
		Store:               st,
		Runner:              runner.New(),
		Timeout:             time.Second,
		CancelCheckInterval: time.Millisecond,
	}.Execute(context.Background(), run.ID)
	if err != nil {
		t.Fatal(err)
	}

	canceled, ok := st.GetRun(run.ID)
	if !ok {
		t.Fatal("run was not found")
	}
	if canceled.Status != store.RunCanceled {
		t.Fatalf("run status = %s, want %s", canceled.Status, store.RunCanceled)
	}
	if canceled.Report == nil || canceled.Report.Status != report.StatusCanceled {
		t.Fatalf("run report = %#v, want canceled report", canceled.Report)
	}
	if requests != 0 {
		t.Fatalf("target requests = %d, want 0", requests)
	}
}

func TestExecutorCancelsWorkerOwnedRunningRun(t *testing.T) {
	started := make(chan struct{}, 1)
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case started <- struct{}{}:
		default:
		}
		<-r.Context().Done()
	}))
	defer target.Close()

	st := store.NewMemory()
	run := st.CreateRun("", "", testRunScenario("cancel-running-worker-run", target.URL, 20))
	errs := make(chan error, 1)
	go func() {
		errs <- Executor{
			Store:               st,
			Runner:              runner.New(),
			Timeout:             time.Second,
			CancelCheckInterval: time.Millisecond,
		}.Execute(context.Background(), run.ID)
	}()

	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("worker run did not start a target request")
	}
	if !st.RequestRunCancel(run.ID) {
		t.Fatal("cancel request was not recorded")
	}

	select {
	case err := <-errs:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("worker run did not stop after cancellation request")
	}

	canceled, ok := st.GetRun(run.ID)
	if !ok {
		t.Fatal("run was not found")
	}
	if canceled.Status != store.RunCanceled {
		t.Fatalf("run status = %s, want %s", canceled.Status, store.RunCanceled)
	}
	if canceled.Report == nil || canceled.Report.Status != report.StatusCanceled {
		t.Fatalf("run report = %#v, want canceled report", canceled.Report)
	}
	if canceled.Report.Summary.TotalRequests == 0 {
		t.Fatal("canceled report did not include the interrupted request")
	}
	if canceled.Report.Summary.TotalRequests >= 20 {
		t.Fatalf("runner completed all requests after cancellation: total requests = %d", canceled.Report.Summary.TotalRequests)
	}
}

func testRunScenario(name string, baseURL string, iterations int) scenario.Scenario {
	return scenario.Scenario{
		Version: 1,
		Name:    name,
		Target:  scenario.Target{BaseURL: baseURL},
		Traffic: scenario.Traffic{
			Type:        scenario.TrafficLoad,
			Concurrency: 1,
			Iterations:  iterations,
		},
		Requests: []scenario.Request{{
			Name:   "request",
			Method: http.MethodGet,
			Path:   "/",
			Expect: scenario.RequestExpectation{Status: []int{http.StatusOK}},
		}},
	}
}
