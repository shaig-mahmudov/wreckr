package runner

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/wreckr/wreckr/apps/api/internal/scenario"
)

func TestK6RunnerReconstructRecords(t *testing.T) {
	summaryJSON := `{
		"metrics": {
			"http_req_duration": {
				"type": "trend",
				"values": {
					"avg": 25.5,
					"min": 10.0,
					"med": 20.0,
					"max": 100.0,
					"p(95)": 80.0,
					"p(99)": 95.0
				}
			},
			"http_reqs{name:\"checkout\",status:201}": {
				"type": "counter",
				"values": {
					"count": 5
				}
			},
			"http_reqs{name:\"checkout\",status:409}": {
				"type": "counter",
				"values": {
					"count": 2
				}
			},
			"http_reqs{name:\"reset\",status:200}": {
				"type": "counter",
				"values": {
					"count": 1
				}
			},
			"http_reqs{name:\"checkout\",status:0}": {
				"type": "counter",
				"values": {
					"count": 1
				}
			}
		}
	}`

	var summary k6Summary
	if err := json.Unmarshal([]byte(summaryJSON), &summary); err != nil {
		t.Fatal(err)
	}

	sc := scenario.Scenario{
		Name: "test-scenario",
	}

	kr := NewK6Runner()
	startedAt := time.Now()
	records := kr.reconstructRecords(sc, &summary, startedAt)

	// We expect 5 + 2 + 1 + 1 = 9 records
	if len(records) != 9 {
		t.Errorf("expected 9 records, got %d", len(records))
	}

	counts := map[string]map[int]int{}
	for _, rec := range records {
		if rec.DurationMS != 25.5 {
			t.Errorf("expected duration 25.5, got %.1f", rec.DurationMS)
		}
		if counts[rec.RequestName] == nil {
			counts[rec.RequestName] = map[int]int{}
		}
		counts[rec.RequestName][rec.StatusCode]++
	}

	if counts["checkout"][201] != 5 {
		t.Errorf("expected 5 checkout 201 records, got %d", counts["checkout"][201])
	}
	if counts["checkout"][409] != 2 {
		t.Errorf("expected 2 checkout 409 records, got %d", counts["checkout"][409])
	}
	if counts["reset"][200] != 1 {
		t.Errorf("expected 1 reset 200 records, got %d", counts["reset"][200])
	}
	if counts["checkout"][0] != 1 {
		t.Errorf("expected 1 checkout 0 records, got %d", counts["checkout"][0])
	}
}

func TestK6RunnerParseThresholds(t *testing.T) {
	summaryJSON := `{
		"metrics": {
			"http_req_duration": {
				"type": "trend",
				"values": {
					"p(95)": 120.5
				},
				"thresholds": {
					"p(95)<=200.0": {
						"ok": true
					},
					"p(95)<=100.0": {
						"ok": false
					}
				}
			},
			"checks": {
				"type": "rate",
				"values": {
					"rate": 0.98
				},
				"thresholds": {
					"rate>=0.9500": {
						"ok": true
					},
					"rate>=0.9900": {
						"ok": false
					}
				}
			}
		}
	}`

	var summary k6Summary
	if err := json.Unmarshal([]byte(summaryJSON), &summary); err != nil {
		t.Fatal(err)
	}

	maxErrorRate1 := float64(0.05) // successRate 0.95
	p95Limit1 := float64(200)

	sc1 := scenario.Scenario{
		Thresholds: scenario.Thresholds{
			MaxErrorRate: &maxErrorRate1,
			P95MS:        &p95Limit1,
		},
	}

	kr := NewK6Runner()
	results1 := kr.parseThresholds(sc1, &summary)
	if len(results1) != 2 {
		t.Fatalf("expected 2 threshold results, got %d", len(results1))
	}

	for _, res := range results1 {
		if !res.Passed {
			t.Errorf("expected threshold %s to pass", res.Name)
		}
	}

	maxErrorRate2 := float64(0.01) // successRate 0.99
	p95Limit2 := float64(100)
	sc2 := scenario.Scenario{
		Thresholds: scenario.Thresholds{
			MaxErrorRate: &maxErrorRate2,
			P95MS:        &p95Limit2,
		},
	}

	results2 := kr.parseThresholds(sc2, &summary)
	if len(results2) != 2 {
		t.Fatalf("expected 2 threshold results, got %d", len(results2))
	}

	for _, res := range results2 {
		if res.Passed {
			t.Errorf("expected threshold %s to fail", res.Name)
		}
	}
}
