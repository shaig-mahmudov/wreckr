package httpapi_test

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/wreckr/wreckr/apps/api/internal/config"
	"github.com/wreckr/wreckr/apps/api/internal/httpapi"
	"github.com/wreckr/wreckr/apps/api/internal/report"
	"github.com/wreckr/wreckr/apps/api/internal/runevent"
	"github.com/wreckr/wreckr/apps/api/internal/runner"
	"github.com/wreckr/wreckr/apps/api/internal/scenario"
	"github.com/wreckr/wreckr/apps/api/internal/store"
)

func TestScenarioCreateAndListEndpoints(t *testing.T) {
	target := newTargetServer(t)
	api := newAPIServer(t)

	sc := testScenario(target.URL)
	created := postJSON[store.ScenarioRecord](t, api.URL+"/v1/scenarios", sc, http.StatusCreated)
	if created.ID == "" {
		t.Fatal("created scenario ID is empty")
	}
	if created.Scenario.Name != sc.Name {
		t.Fatalf("created scenario name = %q, want %q", created.Scenario.Name, sc.Name)
	}

	listed := getJSON[struct {
		Scenarios []store.ScenarioRecord `json:"scenarios"`
	}](t, api.URL+"/v1/scenarios", http.StatusOK)
	if len(listed.Scenarios) != 1 {
		t.Fatalf("listed scenario count = %d, want 1", len(listed.Scenarios))
	}
	if listed.Scenarios[0].ID != created.ID {
		t.Fatalf("listed scenario ID = %q, want %q", listed.Scenarios[0].ID, created.ID)
	}
}

func TestTargetCRUDEndpoints(t *testing.T) {
	targetServer := newTargetServer(t)
	api := newAPIServer(t)

	createPayload := map[string]any{
		"name":        "Local Demo API",
		"baseUrl":     targetServer.URL,
		"environment": "local",
		"description": "Demo target for local runs",
		"headers": map[string]string{
			"X-Wreckr-Target": "local-demo",
		},
	}
	created := postJSON[store.TargetRecord](t, api.URL+"/v1/targets", createPayload, http.StatusCreated)
	if created.ID == "" {
		t.Fatal("created target ID is empty")
	}
	if created.BaseURL != targetServer.URL {
		t.Fatalf("created target baseUrl = %q, want %q", created.BaseURL, targetServer.URL)
	}
	if created.Environment != store.TargetLocal {
		t.Fatalf("created target environment = %q, want %q", created.Environment, store.TargetLocal)
	}
	if created.Headers["X-Wreckr-Target"] != "local-demo" {
		t.Fatalf("created target headers = %#v, want X-Wreckr-Target", created.Headers)
	}

	listed := getJSON[struct {
		Targets []store.TargetRecord `json:"targets"`
	}](t, api.URL+"/v1/targets", http.StatusOK)
	if len(listed.Targets) != 1 {
		t.Fatalf("listed target count = %d, want 1", len(listed.Targets))
	}
	if listed.Targets[0].ID != created.ID {
		t.Fatalf("listed target ID = %q, want %q", listed.Targets[0].ID, created.ID)
	}

	got := getJSON[store.TargetRecord](t, api.URL+"/v1/targets/"+created.ID, http.StatusOK)
	if got.Name != "Local Demo API" {
		t.Fatalf("target name = %q, want Local Demo API", got.Name)
	}

	updatePayload := map[string]any{
		"name":        "Staging Demo API",
		"baseUrl":     targetServer.URL,
		"environment": "staging",
		"description": "Updated target",
	}
	updated := putJSON[store.TargetRecord](t, api.URL+"/v1/targets/"+created.ID, updatePayload, http.StatusOK)
	if updated.Name != "Staging Demo API" {
		t.Fatalf("updated target name = %q, want Staging Demo API", updated.Name)
	}
	if updated.Environment != store.TargetStaging {
		t.Fatalf("updated target environment = %q, want %q", updated.Environment, store.TargetStaging)
	}

	deleteRaw(t, api.URL+"/v1/targets/"+created.ID, http.StatusNoContent)
	assertErrorResponse(t, getRaw(t, api.URL+"/v1/targets/"+created.ID, http.StatusNotFound))
}

func TestRunCreationResolvesExplicitTarget(t *testing.T) {
	var sawTargetHeader bool
	var sawScenarioHeader bool
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Wreckr-Target") == "target-header" {
			sawTargetHeader = true
		}
		if r.Header.Get("X-Wreckr-Custom-Scenario") == "scenario-header" {
			sawScenarioHeader = true
		}
		switch r.URL.Path {
		case "/setup", "/work", "/teardown":
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]any{"ok": true})
		default:
			http.NotFound(w, r)
		}
	}))
	defer target.Close()

	api := newAPIServer(t)
	createdTarget := postJSON[store.TargetRecord](t, api.URL+"/v1/targets", map[string]any{
		"name":        "Resolved Target",
		"baseUrl":     target.URL,
		"environment": "development",
		"headers": map[string]string{
			"X-Wreckr-Target": "target-header",
		},
	}, http.StatusCreated)

	sc := testScenario("http://example.invalid")
	sc.Target.Headers = map[string]string{
		"X-Wreckr-Custom-Scenario": "scenario-header",
	}
	run := postJSON[store.RunRecord](t, api.URL+"/v1/runs", map[string]any{
		"scenario":  sc,
		"target_id": createdTarget.ID,
		"sync":      true,
	}, http.StatusCreated)

	if run.Status != store.RunPassed {
		t.Fatalf("run status = %s, want %s", run.Status, store.RunPassed)
	}
	if run.TargetID != createdTarget.ID {
		t.Fatalf("run target_id = %q, want %q", run.TargetID, createdTarget.ID)
	}
	if run.Scenario.Target.BaseURL != target.URL {
		t.Fatalf("run target base URL = %q, want %q", run.Scenario.Target.BaseURL, target.URL)
	}
	if !sawTargetHeader {
		t.Fatal("target header was not sent to resolved target")
	}
	if !sawScenarioHeader {
		t.Fatal("scenario header was not sent to resolved target")
	}
}

func TestRunCreateStatusAndReportEndpoints(t *testing.T) {
	target := newTargetServer(t)
	api := newAPIServer(t)

	created := postJSON[store.ScenarioRecord](t, api.URL+"/v1/scenarios", testScenario(target.URL), http.StatusCreated)
	run := postJSON[store.RunRecord](t, api.URL+"/v1/runs", map[string]any{
		"scenario_id": created.ID,
		"sync":        true,
	}, http.StatusCreated)
	if run.ID == "" {
		t.Fatal("run ID is empty")
	}
	if run.Status != store.RunPassed {
		t.Fatalf("run status = %s, want %s", run.Status, store.RunPassed)
	}
	if run.Report == nil {
		t.Fatal("sync run did not include a report")
	}

	status := getJSON[store.RunRecord](t, api.URL+"/v1/runs/"+run.ID, http.StatusOK)
	if status.ID != run.ID {
		t.Fatalf("status run ID = %q, want %q", status.ID, run.ID)
	}
	if status.Status != store.RunPassed {
		t.Fatalf("status = %s, want %s", status.Status, store.RunPassed)
	}

	rep := getJSON[report.Report](t, api.URL+"/v1/runs/"+run.ID+"/report", http.StatusOK)
	if rep.Status != report.StatusPassed {
		t.Fatalf("report status = %s, want %s", rep.Status, report.StatusPassed)
	}
	if rep.Summary.TotalRequests != 1 {
		t.Fatalf("report total requests = %d, want 1", rep.Summary.TotalRequests)
	}
}

func TestRunEventsEndpointReturnsChronologicalTimeline(t *testing.T) {
	target := newTargetServer(t)
	api := newAPIServer(t)

	run := postJSON[store.RunRecord](t, api.URL+"/v1/runs", map[string]any{
		"scenario": testScenario(target.URL),
		"sync":     true,
	}, http.StatusCreated)

	events := getJSON[struct {
		Events []runevent.Event `json:"events"`
	}](t, api.URL+"/v1/runs/"+run.ID+"/events", http.StatusOK)
	if len(events.Events) == 0 {
		t.Fatal("expected run events")
	}
	for i, event := range events.Events {
		if event.RunID != run.ID {
			t.Fatalf("events[%d].run_id = %q, want %q", i, event.RunID, run.ID)
		}
		if event.Sequence != int64(i+1) {
			t.Fatalf("events[%d].sequence = %d, want %d", i, event.Sequence, i+1)
		}
	}
	assertEventTypes(t, events.Events,
		runevent.TypeRunQueued,
		runevent.TypeRunStarted,
		runevent.TypeSetupStarted,
		runevent.TypeSetupCompleted,
		runevent.TypeRequestStarted,
		runevent.TypeRequestCompleted,
		runevent.TypeTeardownStarted,
		runevent.TypeTeardownCompleted,
		runevent.TypeRunCompleted,
	)
}

func TestRunEventsRecordAssertionThresholdAndInvariantFailures(t *testing.T) {
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/work" {
			http.NotFound(w, r)
			return
		}
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer target.Close()

	api := newAPIServer(t)
	sc := testScenario(target.URL)
	sc.Setup = nil
	sc.Teardown = nil
	sc.Requests[0].Expect = scenario.RequestExpectation{Status: []int{http.StatusOK}}
	equalsZero := 0
	status500 := http.StatusInternalServerError
	sc.Invariants = []scenario.Invariant{{
		Name:    "no-500s",
		Type:    "response_count",
		Request: "work",
		Status:  &status500,
		Equals:  &equalsZero,
	}}

	run := postJSON[store.RunRecord](t, api.URL+"/v1/runs", map[string]any{
		"scenario": sc,
		"sync":     true,
	}, http.StatusCreated)
	if run.Status != store.RunFailed {
		t.Fatalf("run status = %s, want %s", run.Status, store.RunFailed)
	}

	events := getJSON[struct {
		Events []runevent.Event `json:"events"`
	}](t, api.URL+"/v1/runs/"+run.ID+"/events", http.StatusOK)
	assertEventTypes(t, events.Events,
		runevent.TypeAssertionFailed,
		runevent.TypeThresholdFailed,
		runevent.TypeInvariantFailed,
		runevent.TypeRunFailed,
	)
	if !eventTypeHasMetadata(events.Events, runevent.TypeAssertionFailed, "expected") {
		t.Fatal("assertion_failed event missing structured expected metadata")
	}
}

func TestRunEventsStreamReturnsPersistedEventsAndClosesAtTerminalState(t *testing.T) {
	target := newTargetServer(t)
	api := newAPIServer(t)

	run := postJSON[store.RunRecord](t, api.URL+"/v1/runs", map[string]any{
		"scenario": testScenario(target.URL),
		"sync":     true,
	}, http.StatusCreated)

	events := streamRunEvents(t, api.URL+"/v1/runs/"+run.ID+"/events/stream", "")
	if len(events) == 0 {
		t.Fatal("expected streamed events")
	}
	assertEventTypes(t, events, runevent.TypeRunQueued, runevent.TypeRunStarted, runevent.TypeRunCompleted)
	if events[len(events)-1].Type != runevent.TypeRunCompleted {
		t.Fatalf("last stream event = %s, want %s", events[len(events)-1].Type, runevent.TypeRunCompleted)
	}
}

func TestRunEventsStreamResumesAfterLastEventID(t *testing.T) {
	target := newTargetServer(t)
	api := newAPIServer(t)

	run := postJSON[store.RunRecord](t, api.URL+"/v1/runs", map[string]any{
		"scenario": testScenario(target.URL),
		"sync":     true,
	}, http.StatusCreated)
	persisted := getJSON[struct {
		Events []runevent.Event `json:"events"`
	}](t, api.URL+"/v1/runs/"+run.ID+"/events", http.StatusOK)
	if len(persisted.Events) < 2 {
		t.Fatalf("expected at least two persisted events, got %d", len(persisted.Events))
	}

	events := streamRunEvents(t, api.URL+"/v1/runs/"+run.ID+"/events/stream", strconv.FormatInt(persisted.Events[0].Sequence, 10))
	if len(events) == 0 {
		t.Fatal("expected resumed stream events")
	}
	if events[0].Sequence <= persisted.Events[0].Sequence {
		t.Fatalf("resumed event sequence = %d, want > %d", events[0].Sequence, persisted.Events[0].Sequence)
	}
}

func TestRunCreationWithQueueEnqueuesWithoutInProcessExecution(t *testing.T) {
	target := newTargetServer(t)
	queue := &fakeRunQueue{}
	api := newAPIServerWithConfigAndQueue(t, testConfig(config.Guardrails{
		MaxConcurrency:      1000,
		MaxRequestRate:      5000,
		MaxRunDuration:      5 * time.Second,
		MaxRequestBodyBytes: 1 << 20,
	}), queue)

	run := postJSON[store.RunRecord](t, api.URL+"/v1/runs", map[string]any{
		"scenario": testScenario(target.URL),
		"sync":     true,
	}, http.StatusAccepted)

	if run.Status != store.RunQueued {
		t.Fatalf("run status = %s, want %s", run.Status, store.RunQueued)
	}
	if run.Report != nil {
		t.Fatalf("queued run report = %#v, want nil", run.Report)
	}
	if len(queue.ids) != 1 || queue.ids[0] != run.ID {
		t.Fatalf("queued IDs = %v, want [%s]", queue.ids, run.ID)
	}

	status := getJSON[store.RunRecord](t, api.URL+"/v1/runs/"+run.ID, http.StatusOK)
	if status.Status != store.RunQueued {
		t.Fatalf("status = %s, want %s", status.Status, store.RunQueued)
	}
	assertErrorResponse(t, getRaw(t, api.URL+"/v1/runs/"+run.ID+"/report", http.StatusConflict))
}

func TestCancelWorkerOwnedRunningRunRecordsDurableRequest(t *testing.T) {
	target := newTargetServer(t)
	queue := &fakeRunQueue{}
	st := store.NewMemory()
	api := newAPIServerWithStoreAndQueue(t, testConfig(config.Guardrails{
		MaxConcurrency:      1000,
		MaxRequestRate:      5000,
		MaxRunDuration:      5 * time.Second,
		MaxRequestBodyBytes: 1 << 20,
	}), st, queue)

	run := postJSON[store.RunRecord](t, api.URL+"/v1/runs", map[string]any{
		"scenario": testScenario(target.URL),
	}, http.StatusAccepted)
	st.MarkRunStarted(run.ID)

	canceled := postJSON[struct {
		ID     string          `json:"id"`
		Status store.RunStatus `json:"status"`
	}](t, api.URL+"/v1/runs/"+run.ID+"/cancel", map[string]any{}, http.StatusAccepted)
	if canceled.ID != run.ID {
		t.Fatalf("cancel response ID = %q, want %q", canceled.ID, run.ID)
	}
	if canceled.Status != store.RunCanceled {
		t.Fatalf("cancel response status = %s, want %s", canceled.Status, store.RunCanceled)
	}
	if !st.IsRunCancelRequested(run.ID) {
		t.Fatal("cancel request was not persisted for worker-owned run")
	}
	status := getJSON[store.RunRecord](t, api.URL+"/v1/runs/"+run.ID, http.StatusOK)
	if status.Status != store.RunRunning {
		t.Fatalf("run status = %s, want worker-owned run to remain running until worker observes cancellation", status.Status)
	}
}

func TestScenarioUpdateCreatesVersionWithoutMutatingOldRunReport(t *testing.T) {
	target := newTargetServer(t)
	api := newAPIServer(t)

	created := postJSON[store.ScenarioRecord](t, api.URL+"/v1/scenarios", testScenario(target.URL), http.StatusCreated)
	if created.CurrentVersionNumber != 1 {
		t.Fatalf("created version = %d, want 1", created.CurrentVersionNumber)
	}

	run := postJSON[store.RunRecord](t, api.URL+"/v1/runs", map[string]any{
		"scenario_id": created.ID,
		"sync":        true,
	}, http.StatusCreated)
	if run.ScenarioVersionNumber != 1 {
		t.Fatalf("run scenario version = %d, want 1", run.ScenarioVersionNumber)
	}
	if run.Report == nil || run.Report.ScenarioVersionNumber != 1 {
		t.Fatalf("run report version = %#v, want version 1", run.Report)
	}

	updatedScenario := testScenario(target.URL)
	updatedScenario.Name = "api-integration-scenario-v2"
	updated := putJSON[store.ScenarioRecord](t, api.URL+"/v1/scenarios/"+created.ID, updatedScenario, http.StatusOK)
	if updated.CurrentVersionNumber != 2 {
		t.Fatalf("updated version = %d, want 2", updated.CurrentVersionNumber)
	}

	versions := getJSON[struct {
		Versions []store.ScenarioVersionRecord `json:"versions"`
	}](t, api.URL+"/v1/scenarios/"+created.ID+"/versions", http.StatusOK)
	if len(versions.Versions) != 2 {
		t.Fatalf("version count = %d, want 2", len(versions.Versions))
	}

	oldReport := getJSON[report.Report](t, api.URL+"/v1/runs/"+run.ID+"/report", http.StatusOK)
	if oldReport.ScenarioVersionNumber != 1 {
		t.Fatalf("old report version = %d, want 1", oldReport.ScenarioVersionNumber)
	}
	if oldReport.Scenario != "api-integration-scenario" {
		t.Fatalf("old report scenario = %q, want original scenario name", oldReport.Scenario)
	}
}

func TestRunCancellationEndpointCancelsRunningRunAndSavesPartialReport(t *testing.T) {
	started := make(chan struct{}, 1)
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/slow" {
			http.NotFound(w, r)
			return
		}
		select {
		case started <- struct{}{}:
		default:
		}
		<-r.Context().Done()
	}))
	defer target.Close()

	api := newAPIServer(t)
	sc := testScenario(target.URL)
	sc.Setup = nil
	sc.Teardown = nil
	sc.Traffic.Iterations = 50
	sc.Requests = []scenario.Request{{
		Name:   "slow",
		Method: http.MethodGet,
		Path:   "/slow",
		Expect: scenario.RequestExpectation{Status: []int{http.StatusOK}},
	}}

	run := postJSON[store.RunRecord](t, api.URL+"/v1/runs", map[string]any{
		"scenario": sc,
		"sync":     false,
	}, http.StatusAccepted)

	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("run did not start a target request")
	}

	canceled := postJSON[struct {
		ID     string          `json:"id"`
		Status store.RunStatus `json:"status"`
	}](t, api.URL+"/v1/runs/"+run.ID+"/cancel", map[string]any{}, http.StatusAccepted)
	if canceled.ID != run.ID {
		t.Fatalf("cancel response ID = %q, want %q", canceled.ID, run.ID)
	}
	if canceled.Status != store.RunCanceled {
		t.Fatalf("cancel response status = %s, want %s", canceled.Status, store.RunCanceled)
	}

	finalRun := waitForRunStatus(t, api.URL, run.ID, store.RunCanceled)
	if finalRun.Report == nil {
		t.Fatal("canceled run did not include a partial report")
	}
	if finalRun.Report.Status != report.StatusCanceled {
		t.Fatalf("partial report status = %s, want %s", finalRun.Report.Status, report.StatusCanceled)
	}
	if finalRun.Report.Summary.TotalRequests == 0 {
		t.Fatal("partial report did not include the in-flight request")
	}
	if finalRun.Report.Summary.TotalRequests >= sc.Traffic.Iterations {
		t.Fatalf("runner continued sending requests after cancellation: total requests = %d", finalRun.Report.Summary.TotalRequests)
	}

	rep := getJSON[report.Report](t, api.URL+"/v1/runs/"+run.ID+"/report", http.StatusOK)
	if rep.Status != report.StatusCanceled {
		t.Fatalf("report endpoint status = %s, want %s", rep.Status, report.StatusCanceled)
	}
}

func TestCancelCompletedRunReturnsConflict(t *testing.T) {
	target := newTargetServer(t)
	api := newAPIServer(t)

	run := postJSON[store.RunRecord](t, api.URL+"/v1/runs", map[string]any{
		"scenario": testScenario(target.URL),
		"sync":     true,
	}, http.StatusCreated)
	if run.Status != store.RunPassed {
		t.Fatalf("run status = %s, want %s", run.Status, store.RunPassed)
	}

	resp := postJSONRaw(t, api.URL+"/v1/runs/"+run.ID+"/cancel", map[string]any{}, http.StatusConflict)
	assertErrorResponse(t, resp)
}

func TestRunGuardrailsRejectUnsafeScenarios(t *testing.T) {
	target := newTargetServer(t)

	tests := []struct {
		name      string
		cfg       config.Config
		mutate    func(*scenario.Scenario)
		wantError string
	}{
		{
			name: "max concurrency",
			cfg: testConfig(config.Guardrails{
				MaxConcurrency:      1,
				MaxRequestBodyBytes: 1 << 20,
			}),
			mutate: func(sc *scenario.Scenario) {
				sc.Traffic.Concurrency = 2
			},
			wantError: "traffic.concurrency 2 exceeds max_concurrency 1",
		},
		{
			name: "max request rate",
			cfg: testConfig(config.Guardrails{
				MaxRequestRate:      10,
				MaxRequestBodyBytes: 1 << 20,
			}),
			mutate: func(sc *scenario.Scenario) {
				sc.Traffic.RatePerSecond = 11
			},
			wantError: "traffic.rate_per_second 11 exceeds max_request_rate_per_second 10",
		},
		{
			name: "max request body size",
			cfg: testConfig(config.Guardrails{
				MaxRequestBodyBytes: 10,
			}),
			mutate: func(sc *scenario.Scenario) {
				sc.Requests[0].Body = strings.Repeat("x", 11)
			},
			wantError: "requests[0].body size 11 exceeds max_request_body_bytes 10",
		},
		{
			name: "target allowlist",
			cfg: testConfig(config.Guardrails{
				MaxRequestBodyBytes: 1 << 20,
				TargetAllowlist:     []string{"allowed.example"},
			}),
			wantError: "target.base_url host",
		},
		{
			name: "absolute request target",
			cfg: testConfig(config.Guardrails{
				MaxRequestBodyBytes: 1 << 20,
			}),
			mutate: func(sc *scenario.Scenario) {
				sc.Requests[0].Path = "http://evil.example/work"
			},
			wantError: "requests[0].path absolute URL host",
		},
		{
			name: "target credentials",
			cfg: testConfig(config.Guardrails{
				MaxRequestBodyBytes: 1 << 20,
			}),
			mutate: func(sc *scenario.Scenario) {
				sc.Target.BaseURL = "http://user:pass@example.com"
			},
			wantError: "target.base_url must not include credentials",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			api := newAPIServerWithConfig(t, tc.cfg)
			sc := testScenario(target.URL)
			if tc.mutate != nil {
				tc.mutate(&sc)
			}
			resp := postJSONRaw(t, api.URL+"/v1/runs", map[string]any{
				"scenario": sc,
				"sync":     true,
			}, http.StatusBadRequest)
			assertErrorContains(t, resp, tc.wantError)
		})
	}
}

func TestRunGuardrailsAllowlistedTargetCanRun(t *testing.T) {
	target := newTargetServer(t)
	cfg := testConfig(config.Guardrails{
		MaxConcurrency:      10,
		MaxRequestRate:      100,
		MaxRequestBodyBytes: 1 << 20,
		TargetAllowlist:     []string{mustURLHost(t, target.URL)},
	})
	api := newAPIServerWithConfig(t, cfg)

	run := postJSON[store.RunRecord](t, api.URL+"/v1/runs", map[string]any{
		"scenario": testScenario(target.URL),
		"sync":     true,
	}, http.StatusCreated)
	if run.Status != store.RunPassed {
		t.Fatalf("run status = %s, want %s", run.Status, store.RunPassed)
	}
}

func TestRunGuardrailsApplyMaxRequestRateWhenScenarioOmitsRate(t *testing.T) {
	target := newTargetServer(t)
	api := newAPIServerWithConfig(t, testConfig(config.Guardrails{
		MaxRequestRate:      10,
		MaxRequestBodyBytes: 1 << 20,
	}))
	sc := testScenario(target.URL)
	sc.Setup = nil
	sc.Teardown = nil
	sc.Traffic.Concurrency = 2
	sc.Traffic.Iterations = 2

	startedAt := time.Now()
	run := postJSON[store.RunRecord](t, api.URL+"/v1/runs", map[string]any{
		"scenario": sc,
		"sync":     true,
	}, http.StatusCreated)
	elapsed := time.Since(startedAt)
	if run.Status != store.RunPassed {
		t.Fatalf("run status = %s, want %s", run.Status, store.RunPassed)
	}
	if run.Scenario.Traffic.RatePerSecond != 10 {
		t.Fatalf("run rate_per_second = %d, want guardrail cap 10", run.Scenario.Traffic.RatePerSecond)
	}
	if elapsed < 150*time.Millisecond {
		t.Fatalf("run completed too quickly for guardrail rate cap: elapsed = %s", elapsed)
	}
}

func TestRunGuardrailsMaxRunDurationStopsRun(t *testing.T) {
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))
	defer target.Close()

	api := newAPIServerWithConfig(t, testConfig(config.Guardrails{
		MaxRunDuration:      20 * time.Millisecond,
		MaxRequestBodyBytes: 1 << 20,
	}))
	sc := testScenario(target.URL)
	sc.Setup = nil
	sc.Teardown = nil

	startedAt := time.Now()
	run := postJSON[store.RunRecord](t, api.URL+"/v1/runs", map[string]any{
		"scenario": sc,
		"sync":     true,
	}, http.StatusCreated)
	if elapsed := time.Since(startedAt); elapsed > time.Second {
		t.Fatalf("run duration guardrail did not stop the run quickly: elapsed = %s", elapsed)
	}
	if run.Status != store.RunFailed {
		t.Fatalf("run status = %s, want %s", run.Status, store.RunFailed)
	}
	if run.Report == nil || len(run.Report.Responses) != 1 {
		t.Fatalf("expected partial failed report, got %#v", run.Report)
	}
	if !strings.Contains(run.Report.Responses[0].Error, "context deadline exceeded") {
		t.Fatalf("response error = %q, want context deadline exceeded", run.Report.Responses[0].Error)
	}
}

func TestRunCreationWithInlineScenario(t *testing.T) {
	target := newTargetServer(t)
	api := newAPIServer(t)

	run := postJSON[store.RunRecord](t, api.URL+"/v1/runs", map[string]any{
		"scenario": testScenario(target.URL),
		"sync":     true,
	}, http.StatusCreated)

	if run.Status != store.RunPassed {
		t.Fatalf("run status = %s, want %s", run.Status, store.RunPassed)
	}
	if run.Report == nil || run.Report.Status != report.StatusPassed {
		t.Fatalf("expected passed report, got %#v", run.Report)
	}
}

func TestInvalidInputReturnsErrorResponses(t *testing.T) {
	api := newAPIServer(t)

	assertErrorResponse(t, postRaw(t, api.URL+"/v1/scenarios", []byte(`{"bad":true}`), http.StatusBadRequest))
	assertErrorResponse(t, postJSONRaw(t, api.URL+"/v1/runs", map[string]any{}, http.StatusBadRequest))
	assertErrorResponse(t, getRaw(t, api.URL+"/v1/scenarios/missing", http.StatusNotFound))
	assertErrorResponse(t, getRaw(t, api.URL+"/v1/runs/missing", http.StatusNotFound))
}

func TestReportEndpointReturnsConflictWhenReportIsNotReady(t *testing.T) {
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(100 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer target.Close()

	api := newAPIServer(t)
	run := postJSON[store.RunRecord](t, api.URL+"/v1/runs", map[string]any{
		"scenario": testScenario(target.URL),
		"sync":     false,
	}, http.StatusAccepted)

	resp := getRaw(t, api.URL+"/v1/runs/"+run.ID+"/report", http.StatusConflict)
	assertErrorResponse(t, resp)
}

func newAPIServer(t *testing.T) *httptest.Server {
	t.Helper()
	return newAPIServerWithConfig(t, testConfig(config.Guardrails{
		MaxConcurrency:      1000,
		MaxRequestRate:      5000,
		MaxRunDuration:      5 * time.Second,
		MaxRequestBodyBytes: 1 << 20,
	}))
}

func newAPIServerWithConfig(t *testing.T, cfg config.Config) *httptest.Server {
	t.Helper()
	return newAPIServerWithConfigAndQueue(t, cfg, nil)
}

func newAPIServerWithConfigAndQueue(t *testing.T, cfg config.Config, queue fakeQueue) *httptest.Server {
	t.Helper()
	return newAPIServerWithStoreAndQueue(t, cfg, store.NewMemory(), queue)
}

func newAPIServerWithStoreAndQueue(t *testing.T, cfg config.Config, st store.Store, queue fakeQueue) *httptest.Server {
	t.Helper()
	srv := httpapi.New(config.Config{
		Addr:         cfg.Addr,
		RunTimeout:   cfg.RunTimeout,
		MaxBodyBytes: cfg.MaxBodyBytes,
		Guardrails:   cfg.Guardrails,
	}, st, runner.New())
	if queue != nil {
		srv = httpapi.NewWithQueue(config.Config{
			Addr:         cfg.Addr,
			RunTimeout:   cfg.RunTimeout,
			MaxBodyBytes: cfg.MaxBodyBytes,
			Guardrails:   cfg.Guardrails,
		}, st, runner.New(), queue)
	}

	api := httptest.NewServer(srv.Handler())
	t.Cleanup(api.Close)
	return api
}

func testConfig(guardrails config.Guardrails) config.Config {
	return config.Config{
		Addr:         ":0",
		RunTimeout:   5 * time.Second,
		MaxBodyBytes: 1 << 20,
		Guardrails:   guardrails,
	}
}

func newTargetServer(t *testing.T) *httptest.Server {
	t.Helper()

	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/setup", "/work", "/teardown":
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]any{"ok": true})
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(target.Close)
	return target
}

func testScenario(baseURL string) scenario.Scenario {
	maxErrorRate := float64(0)
	return scenario.Scenario{
		Version: 1,
		Name:    "api-integration-scenario",
		Target:  scenario.Target{BaseURL: baseURL},
		Traffic: scenario.Traffic{
			Type:        scenario.TrafficLoad,
			Concurrency: 1,
			Iterations:  1,
		},
		Setup: []scenario.Request{{
			Name:   "setup",
			Method: http.MethodPost,
			Path:   "/setup",
			Expect: scenario.RequestExpectation{Status: []int{http.StatusOK}},
		}},
		Requests: []scenario.Request{{
			Name:   "work",
			Method: http.MethodGet,
			Path:   "/work",
			Expect: scenario.RequestExpectation{Status: []int{http.StatusOK}},
		}},
		Teardown: []scenario.Request{{
			Name:   "teardown",
			Method: http.MethodPost,
			Path:   "/teardown",
			Expect: scenario.RequestExpectation{Status: []int{http.StatusOK}},
		}},
		Thresholds: scenario.Thresholds{MaxErrorRate: &maxErrorRate},
	}
}

func postJSON[T any](t *testing.T, url string, payload any, wantStatus int) T {
	t.Helper()
	raw := postJSONRaw(t, url, payload, wantStatus)
	var out T
	decodeJSON(t, raw, &out)
	return out
}

func postJSONRaw(t *testing.T, url string, payload any, wantStatus int) []byte {
	t.Helper()
	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	return postRaw(t, url, raw, wantStatus)
}

func postRaw(t *testing.T, url string, raw []byte, wantStatus int) []byte {
	t.Helper()
	resp, err := http.Post(url, "application/json", bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	return readResponse(t, resp, wantStatus)
}

func putJSON[T any](t *testing.T, url string, payload any, wantStatus int) T {
	t.Helper()
	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	req, err := http.NewRequest(http.MethodPut, url, bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body := readResponse(t, resp, wantStatus)
	var out T
	decodeJSON(t, body, &out)
	return out
}

func getJSON[T any](t *testing.T, url string, wantStatus int) T {
	t.Helper()
	raw := getRaw(t, url, wantStatus)
	var out T
	decodeJSON(t, raw, &out)
	return out
}

func getRaw(t *testing.T, url string, wantStatus int) []byte {
	t.Helper()
	resp, err := http.Get(url)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	return readResponse(t, resp, wantStatus)
}

func deleteRaw(t *testing.T, url string, wantStatus int) []byte {
	t.Helper()
	req, err := http.NewRequest(http.MethodDelete, url, nil)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	return readResponse(t, resp, wantStatus)
}

func streamRunEvents(t *testing.T, url string, lastEventID string) []runevent.Event {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		t.Fatal(err)
	}
	if lastEventID != "" {
		req.Header.Set("Last-Event-ID", lastEventID)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("stream status = %d, want 200; body: %s", resp.StatusCode, string(body))
	}
	if got := resp.Header.Get("Content-Type"); !strings.HasPrefix(got, "text/event-stream") {
		t.Fatalf("content-type = %q, want text/event-stream", got)
	}

	var events []runevent.Event
	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		var event runevent.Event
		if err := json.Unmarshal([]byte(strings.TrimPrefix(line, "data: ")), &event); err != nil {
			t.Fatal(err)
		}
		events = append(events, event)
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
	return events
}

func waitForRunStatus(t *testing.T, baseURL string, id string, status store.RunStatus) store.RunRecord {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		run := getJSON[store.RunRecord](t, baseURL+"/v1/runs/"+id, http.StatusOK)
		if run.Status == status {
			return run
		}
		time.Sleep(10 * time.Millisecond)
	}
	run := getJSON[store.RunRecord](t, baseURL+"/v1/runs/"+id, http.StatusOK)
	t.Fatalf("run status = %s, want %s", run.Status, status)
	return store.RunRecord{}
}

func readResponse(t *testing.T, resp *http.Response, wantStatus int) []byte {
	t.Helper()
	if resp.StatusCode != wantStatus {
		t.Fatalf("status = %d, want %d", resp.StatusCode, wantStatus)
	}
	var body bytes.Buffer
	if _, err := io.Copy(&body, resp.Body); err != nil {
		t.Fatal(err)
	}
	return body.Bytes()
}

func decodeJSON(t *testing.T, raw []byte, dst any) {
	t.Helper()
	if err := json.Unmarshal(raw, dst); err != nil {
		t.Fatalf("decode response JSON: %v\nbody: %s", err, string(raw))
	}
}

func assertErrorResponse(t *testing.T, raw []byte) {
	t.Helper()
	var payload struct {
		Error string `json:"error"`
	}
	decodeJSON(t, raw, &payload)
	if payload.Error == "" {
		t.Fatalf("expected JSON error response, got %s", string(raw))
	}
}

func assertErrorContains(t *testing.T, raw []byte, want string) {
	t.Helper()
	var payload struct {
		Error string `json:"error"`
	}
	decodeJSON(t, raw, &payload)
	if !strings.Contains(payload.Error, want) {
		t.Fatalf("error = %q, want to contain %q", payload.Error, want)
	}
}

func assertEventTypes(t *testing.T, events []runevent.Event, want ...runevent.Type) {
	t.Helper()
	seen := map[runevent.Type]bool{}
	for _, event := range events {
		seen[event.Type] = true
	}
	for _, eventType := range want {
		if !seen[eventType] {
			t.Fatalf("event type %q was not recorded; got %v", eventType, eventTypes(events))
		}
	}
}

func eventTypes(events []runevent.Event) []runevent.Type {
	out := make([]runevent.Type, 0, len(events))
	for _, event := range events {
		out = append(out, event.Type)
	}
	return out
}

func eventTypeHasMetadata(events []runevent.Event, eventType runevent.Type, key string) bool {
	for _, event := range events {
		if event.Type == eventType {
			_, ok := event.Metadata[key]
			return ok
		}
	}
	return false
}

func mustURLHost(t *testing.T, raw string) string {
	t.Helper()
	parsed, err := url.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	return parsed.Hostname()
}

type fakeQueue interface {
	EnqueueRun(ctx context.Context, runID string) error
}

type fakeRunQueue struct {
	ids []string
}

func (q *fakeRunQueue) EnqueueRun(_ context.Context, runID string) error {
	q.ids = append(q.ids, runID)
	return nil
}
