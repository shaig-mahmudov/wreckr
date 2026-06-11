package runner

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"slices"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/wreckr/wreckr/apps/api/internal/report"
	"github.com/wreckr/wreckr/apps/api/internal/scenario"
)

func TestRunnerDetectsBrokenIdempotency(t *testing.T) {
	var mu sync.Mutex
	var orders []map[string]any

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/checkout":
			mu.Lock()
			orders = append(orders, map[string]any{"id": len(orders) + 1, "userId": "user-123", "sku": "item-abc"})
			mu.Unlock()
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(map[string]any{"ok": true})
		case "/orders":
			mu.Lock()
			count := len(orders)
			mu.Unlock()
			_ = json.NewEncoder(w).Encode(map[string]any{"count": count})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	one := 1
	p95 := float64(500)
	maxErrors := float64(0)
	sc := scenario.Scenario{
		Version: 1,
		Name:    "checkout-idempotency-race",
		Target:  scenario.Target{BaseURL: server.URL},
		Traffic: scenario.Traffic{Type: scenario.TrafficRace, Concurrency: 2, Iterations: 1},
		Requests: []scenario.Request{{
			Name:   "checkout",
			Method: "POST",
			Path:   "/checkout",
			JSON:   json.RawMessage(`{"userId":"user-123","sku":"item-abc","quantity":1}`),
			Expect: scenario.RequestExpectation{Status: []int{http.StatusCreated}},
		}},
		Thresholds: scenario.Thresholds{MaxErrorRate: &maxErrors, P95MS: &p95},
		Invariants: []scenario.Invariant{{
			Name: "only-one-order-created",
			Type: "http_probe",
			Path: "/orders?userId=user-123&sku=item-abc",
			Expect: scenario.ProbeExpectation{
				JSONPath: "$.count",
				Equals:   float64(one),
			},
		}},
	}

	got, err := New().Run(context.Background(), sc)
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != report.StatusFailed {
		t.Fatalf("status = %s, want failed", got.Status)
	}
	if len(got.Invariants) != 1 || got.Invariants[0].Passed {
		t.Fatalf("expected failed invariant, got %#v", got.Invariants)
	}
}

func TestRunnerRetryStormRetriesFailedRequests(t *testing.T) {
	var attempts atomic.Int32
	var seenAttempts []string
	var mu sync.Mutex

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		seenAttempts = append(seenAttempts, r.Header.Get("X-Wreckr-Attempt"))
		mu.Unlock()

		attempt := attempts.Add(1)
		if attempt < 3 {
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte(`{"ok":false}`))
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()

	sc := baseScenario(server.URL, scenario.TrafficRetryStorm, 1, 1)
	sc.Traffic.Retry.Attempts = 3
	sc.Requests[0].Path = "/unstable"
	sc.Requests[0].Expect.Status = []int{http.StatusOK}

	got, err := New().Run(context.Background(), sc)
	if err != nil {
		t.Fatal(err)
	}

	if attempts.Load() != 3 {
		t.Fatalf("attempts = %d, want 3", attempts.Load())
	}
	if got.Summary.TotalRequests != 3 {
		t.Fatalf("total requests = %d, want 3", got.Summary.TotalRequests)
	}
	if got.Summary.FailedRequests != 2 {
		t.Fatalf("failed requests = %d, want 2", got.Summary.FailedRequests)
	}
	if !slices.Equal(seenAttempts, []string{"1", "2", "3"}) {
		t.Fatalf("attempt headers = %v, want [1 2 3]", seenAttempts)
	}
	if got.Status != report.StatusFailed {
		t.Fatalf("status = %s, want failed because the first two retry attempts failed validation", got.Status)
	}
}

func TestRunnerCoversLoadBurstAndSpikeTrafficModes(t *testing.T) {
	tests := []scenario.TrafficType{
		scenario.TrafficLoad,
		scenario.TrafficBurst,
		scenario.TrafficSpike,
	}

	for _, trafficType := range tests {
		t.Run(string(trafficType), func(t *testing.T) {
			var requests atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				requests.Add(1)
				w.WriteHeader(http.StatusOK)
			}))
			defer server.Close()

			sc := baseScenario(server.URL, trafficType, 3, 7)
			got, err := New().Run(context.Background(), sc)
			if err != nil {
				t.Fatal(err)
			}

			if got.Status != report.StatusPassed {
				t.Fatalf("status = %s, want passed; failures: %v", got.Status, got.Failures)
			}
			if got.Summary.TotalRequests != 7 {
				t.Fatalf("summary total requests = %d, want 7", got.Summary.TotalRequests)
			}
			if requests.Load() != 7 {
				t.Fatalf("server requests = %d, want 7", requests.Load())
			}
		})
	}
}

func TestRunnerValidatesExpectedHTTPStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusAccepted)
	}))
	defer server.Close()

	sc := baseScenario(server.URL, scenario.TrafficLoad, 1, 1)
	sc.Requests[0].Expect.Status = []int{http.StatusOK}

	got, err := New().Run(context.Background(), sc)
	if err != nil {
		t.Fatal(err)
	}

	if got.Status != report.StatusFailed {
		t.Fatalf("status = %s, want failed", got.Status)
	}
	if got.Summary.FailedRequests != 1 {
		t.Fatalf("failed requests = %d, want 1", got.Summary.FailedRequests)
	}
	if len(got.Responses) != 1 || got.Responses[0].Error == "" {
		t.Fatalf("expected response validation error, got %#v", got.Responses)
	}
}

func TestRunnerRunsSetupAndTeardownHooksInOrder(t *testing.T) {
	var mu sync.Mutex
	var order []string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		order = append(order, r.URL.Path)
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	sc := baseScenario(server.URL, scenario.TrafficLoad, 1, 1)
	sc.Setup = []scenario.Request{{
		Name:   "setup-state",
		Method: http.MethodPost,
		Path:   "/setup",
		Expect: scenario.RequestExpectation{Status: []int{http.StatusOK}},
	}}
	sc.Requests[0].Path = "/work"
	sc.Teardown = []scenario.Request{{
		Name:   "teardown-state",
		Method: http.MethodPost,
		Path:   "/teardown",
		Expect: scenario.RequestExpectation{Status: []int{http.StatusOK}},
	}}

	got, err := New().Run(context.Background(), sc)
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != report.StatusPassed {
		t.Fatalf("status = %s, want passed; failures: %v", got.Status, got.Failures)
	}

	want := []string{"/setup", "/work", "/teardown"}
	if !slices.Equal(order, want) {
		t.Fatalf("request order = %v, want %v", order, want)
	}
}

func TestRunnerReportsThresholdFailures(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	maxErrorRate := float64(0)
	sc := baseScenario(server.URL, scenario.TrafficLoad, 1, 1)
	sc.Thresholds.MaxErrorRate = &maxErrorRate

	got, err := New().Run(context.Background(), sc)
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != report.StatusFailed {
		t.Fatalf("status = %s, want failed", got.Status)
	}
	if len(got.Thresholds) != 1 {
		t.Fatalf("threshold count = %d, want 1", len(got.Thresholds))
	}
	if got.Thresholds[0].Name != "max_error_rate" || got.Thresholds[0].Passed {
		t.Fatalf("expected failed max_error_rate threshold, got %#v", got.Thresholds)
	}
	if len(got.Failures) == 0 {
		t.Fatal("expected threshold failure to be included in report failures")
	}
}

func TestRunnerContextTimeoutStopsRunSafely(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(100 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	sc := baseScenario(server.URL, scenario.TrafficLoad, 1, 100)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	startedAt := time.Now()
	got, err := New().Run(ctx, sc)
	elapsed := time.Since(startedAt)
	if err != nil {
		t.Fatal(err)
	}
	if elapsed > time.Second {
		t.Fatalf("run took %s after context timeout, want under 1s", elapsed)
	}
	if got.Status != report.StatusFailed {
		t.Fatalf("status = %s, want failed due to timeout", got.Status)
	}
	if got.Summary.TotalRequests == 0 {
		t.Fatal("expected at least one attempted request before timeout")
	}
	if got.Summary.TotalRequests >= sc.Traffic.Iterations {
		t.Fatalf("total requests = %d, want less than %d after timeout", got.Summary.TotalRequests, sc.Traffic.Iterations)
	}
	if got.Summary.FailedRequests == 0 {
		t.Fatal("expected timed out request to be recorded as failed")
	}
}

func baseScenario(baseURL string, trafficType scenario.TrafficType, concurrency int, iterations int) scenario.Scenario {
	return scenario.Scenario{
		Version: 1,
		Name:    fmt.Sprintf("%s-test", trafficType),
		Target:  scenario.Target{BaseURL: baseURL},
		Traffic: scenario.Traffic{
			Type:        trafficType,
			Concurrency: concurrency,
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
