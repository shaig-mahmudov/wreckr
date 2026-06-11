package runner

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

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
