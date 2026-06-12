package httpapi_test

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/wreckr/wreckr/apps/api/internal/config"
	"github.com/wreckr/wreckr/apps/api/internal/httpapi"
	"github.com/wreckr/wreckr/apps/api/internal/report"
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

	srv := httpapi.New(config.Config{
		Addr:         ":0",
		RunTimeout:   5 * time.Second,
		MaxBodyBytes: 1 << 20,
	}, store.NewMemory(), runner.New())

	api := httptest.NewServer(srv.Handler())
	t.Cleanup(api.Close)
	return api
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
