package worker

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/wreckr/wreckr/apps/api/internal/report"
	"github.com/wreckr/wreckr/apps/api/internal/runevent"
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
	}, Events: st}
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
	events := st.ListRunEvents(run.ID)
	assertEventTypesContain(t, events, runevent.TypeRunQueued, runevent.TypeWorkerAttemptStarted, runevent.TypeRunStarted, runevent.TypeRunCompleted)
}

func TestHandlerRecordsRetryScheduledAfterFailedAttempt(t *testing.T) {
	st := store.NewMemory()
	run := st.CreateRun("", "", workerEventScenario("worker-retry"))
	task, err := runqueue.NewRunTask(run.ID)
	if err != nil {
		t.Fatal(err)
	}

	handler := Handler{
		Executor: failingExecutor{err: errors.New("temporary worker failure")},
		Events:   st,
		TaskInfo: fixedTaskInfo(1, 3),
	}
	err = handler.handleRunExecute(context.Background(), task)
	if err == nil {
		t.Fatal("handleRunExecute returned nil, want error")
	}
	handler.HandleError(context.Background(), task, err)

	events := st.ListRunEvents(run.ID)
	assertEventTypes(t,
		events,
		runevent.TypeRunQueued,
		runevent.TypeWorkerAttemptStarted,
		runevent.TypeWorkerAttemptFailed,
		runevent.TypeWorkerRetryScheduled,
	)
	last := events[len(events)-1]
	if got := last.Metadata["attempt"]; got != 2 {
		t.Fatalf("attempt metadata = %#v, want 2", got)
	}
	if got := last.Metadata["max_retries"]; got != 3 {
		t.Fatalf("max_retries metadata = %#v, want 3", got)
	}
}

func TestHandlerRecordsDeadLetterAfterFinalFailedAttempt(t *testing.T) {
	st := store.NewMemory()
	run := st.CreateRun("", "", workerEventScenario("worker-dead-letter"))
	task, err := runqueue.NewRunTask(run.ID)
	if err != nil {
		t.Fatal(err)
	}

	handler := Handler{
		Executor: failingExecutor{err: errors.New("permanent worker failure")},
		Events:   st,
		TaskInfo: fixedTaskInfo(3, 3),
	}
	err = handler.handleRunExecute(context.Background(), task)
	if err == nil {
		t.Fatal("handleRunExecute returned nil, want error")
	}
	handler.HandleError(context.Background(), task, err)

	events := st.ListRunEvents(run.ID)
	assertEventTypes(t,
		events,
		runevent.TypeRunQueued,
		runevent.TypeWorkerAttemptStarted,
		runevent.TypeWorkerAttemptFailed,
		runevent.TypeWorkerDeadLettered,
	)
	last := events[len(events)-1]
	if last.Metadata["error"] != "permanent worker failure" {
		t.Fatalf("dead-letter error metadata = %#v", last.Metadata["error"])
	}
	if got := last.Metadata["attempt"]; got != 4 {
		t.Fatalf("attempt metadata = %#v, want 4", got)
	}
}

type failingExecutor struct {
	err error
}

func (e failingExecutor) Execute(context.Context, string) error {
	return e.err
}

func fixedTaskInfo(retryCount int, maxRetry int) func(context.Context) TaskInfo {
	return func(context.Context) TaskInfo {
		return TaskInfo{
			TaskID:     "task_test",
			Queue:      runqueue.QueueRuns,
			RetryCount: retryCount,
			MaxRetry:   maxRetry,
			Known:      true,
		}
	}
}

func workerEventScenario(name string) scenario.Scenario {
	return scenario.Scenario{
		Version: 1,
		Name:    name,
		Target:  scenario.Target{BaseURL: "http://example.test"},
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
	}
}

func assertEventTypes(t *testing.T, events []runevent.Event, want ...runevent.Type) {
	t.Helper()
	if len(events) != len(want) {
		t.Fatalf("event count = %d, want %d: %#v", len(events), len(want), eventTypes(events))
	}
	for i, event := range events {
		if event.Type != want[i] {
			t.Fatalf("event[%d] = %s, want %s: %#v", i, event.Type, want[i], eventTypes(events))
		}
	}
}

func assertEventTypesContain(t *testing.T, events []runevent.Event, want ...runevent.Type) {
	t.Helper()
	next := 0
	for _, event := range events {
		if next < len(want) && event.Type == want[next] {
			next++
		}
	}
	if next != len(want) {
		t.Fatalf("events did not contain ordered types %#v: %#v", want, eventTypes(events))
	}
}

func eventTypes(events []runevent.Event) []runevent.Type {
	types := make([]runevent.Type, 0, len(events))
	for _, event := range events {
		types = append(types, event.Type)
	}
	return types
}
