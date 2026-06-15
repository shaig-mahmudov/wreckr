package k6script

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/wreckr/wreckr/apps/api/internal/scenario"
)

func TestCompileGeneratesDeterministicK6Script(t *testing.T) {
	maxErrorRate := float64(0)
	p95 := float64(750)
	sc := scenario.Scenario{
		Version: 1,
		Name:    "checkout-idempotency-race",
		Target: scenario.Target{
			BaseURL: "http://localhost:9090",
			Headers: map[string]string{
				"X-Target": "demo",
			},
		},
		Traffic: scenario.Traffic{
			Type:        scenario.TrafficRace,
			Concurrency: 2,
			Iterations:  3,
		},
		Setup: []scenario.Request{{
			Name:   "reset",
			Method: "POST",
			Path:   "/reset",
			Expect: scenario.RequestExpectation{Status: []int{200}},
		}},
		Requests: []scenario.Request{{
			Name:   "checkout",
			Method: "POST",
			Path:   "/checkout",
			Headers: map[string]string{
				"Idempotency-Key": "same-key",
			},
			JSON:   json.RawMessage(`{"quantity":1,"sku":"item-abc"}`),
			Expect: scenario.RequestExpectation{Status: []int{201, 409}},
		}},
		Thresholds: scenario.Thresholds{
			MaxErrorRate: &maxErrorRate,
			P95MS:        &p95,
		},
		Invariants: []scenario.Invariant{{
			Name:   "only-one-order",
			Type:   "http_probe",
			Method: "GET",
			Path:   "/orders?sku=item-abc",
			Expect: scenario.ProbeExpectation{
				JSONPath: "$.count",
				Equals:   float64(1),
			},
		}},
	}

	first, err := Compile(sc)
	if err != nil {
		t.Fatal(err)
	}
	second, err := Compile(sc)
	if err != nil {
		t.Fatal(err)
	}
	if first.Content != second.Content {
		t.Fatal("compiled script is not deterministic")
	}

	assertContains(t, first.Content, "import http from 'k6/http';")
	assertContains(t, first.Content, "executor: 'shared-iterations'")
	assertContains(t, first.Content, "vus: 2")
	assertContains(t, first.Content, "iterations: 6")
	assertContains(t, first.Content, `"http_req_duration": [`)
	assertContains(t, first.Content, `"p(95)<=750.0"`)
	assertContains(t, first.Content, `"checks": [`)
	assertContains(t, first.Content, `"rate>=1.0000"`)
	assertContains(t, first.Content, `const BASE_URL = "http://localhost:9090";`)
	assertContains(t, first.Content, `"X-Target": "demo"`)
	assertContains(t, first.Content, `export function setup()`)
	assertContains(t, first.Content, `export default function ()`)
	assertContains(t, first.Content, `"name": "checkout"`)
	assertContains(t, first.Content, `"expected_statuses": [`)
	assertContains(t, first.Content, `201`)
	assertContains(t, first.Content, `409`)
	assertContains(t, first.Content, `// - only-one-order: http_probe GET /orders?sku=item-abc with json_path "$.count"`)
}

func TestCompileRetryStormIncludesAttemptsAndBackoff(t *testing.T) {
	sc := scenario.Scenario{
		Version: 1,
		Name:    "retry-storm",
		Target:  scenario.Target{BaseURL: "http://localhost:9090"},
		Traffic: scenario.Traffic{
			Type:        scenario.TrafficRetryStorm,
			Concurrency: 1,
			Iterations:  1,
			Retry: scenario.RetryPolicy{
				Attempts:  4,
				BackoffMS: 250,
			},
		},
		Requests: []scenario.Request{{
			Name:   "work",
			Method: "GET",
			Path:   "/work",
		}},
	}

	script, err := Compile(sc)
	if err != nil {
		t.Fatal(err)
	}
	assertContains(t, script.Content, "for (let attempt = 0; attempt < 4; attempt += 1)")
	assertContains(t, script.Content, "sleep(0.250);")
}

func TestCompileRejectsInvalidScenario(t *testing.T) {
	_, err := Compile(scenario.Scenario{Name: "invalid"})
	if err == nil {
		t.Fatal("expected invalid scenario error")
	}
}

func assertContains(t *testing.T, haystack string, needle string) {
	t.Helper()
	if !strings.Contains(haystack, needle) {
		t.Fatalf("compiled script did not contain %q\nscript:\n%s", needle, haystack)
	}
}
