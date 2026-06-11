package httpapi_test

import (
	"bytes"
	"encoding/json"
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

func readResponse(t *testing.T, resp *http.Response, wantStatus int) []byte {
	t.Helper()
	if resp.StatusCode != wantStatus {
		t.Fatalf("status = %d, want %d", resp.StatusCode, wantStatus)
	}
	var body bytes.Buffer
	if _, err := body.ReadFrom(resp.Body); err != nil {
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
