package report

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"
)

type Status string

const (
	StatusPassed Status = "passed"
	StatusFailed Status = "failed"
)

type ResponseRecord struct {
	RequestName string    `json:"request_name"`
	Iteration   int       `json:"iteration"`
	Attempt     int       `json:"attempt"`
	StatusCode  int       `json:"status_code"`
	DurationMS  float64   `json:"duration_ms"`
	Error       string    `json:"error,omitempty"`
	BodyPreview string    `json:"body_preview,omitempty"`
	StartedAt   time.Time `json:"started_at"`
}

type Summary struct {
	TotalRequests  int            `json:"total_requests"`
	FailedRequests int            `json:"failed_requests"`
	ErrorRate      float64        `json:"error_rate"`
	StatusCodes    map[string]int `json:"status_codes"`
	Latency        LatencyStats   `json:"latency"`
}

type LatencyStats struct {
	MinMS float64 `json:"min_ms"`
	AvgMS float64 `json:"avg_ms"`
	P50MS float64 `json:"p50_ms"`
	P95MS float64 `json:"p95_ms"`
	P99MS float64 `json:"p99_ms"`
	MaxMS float64 `json:"max_ms"`
}

type CheckResult struct {
	Name     string `json:"name"`
	Passed   bool   `json:"passed"`
	Message  string `json:"message"`
	Expected string `json:"expected,omitempty"`
	Actual   string `json:"actual,omitempty"`
}

type Report struct {
	RunID                 string           `json:"run_id"`
	Scenario              string           `json:"scenario"`
	ScenarioVersionID     string           `json:"scenario_version_id,omitempty"`
	ScenarioVersionNumber int              `json:"scenario_version_number,omitempty"`
	Status                Status           `json:"status"`
	StartedAt             time.Time        `json:"started_at"`
	FinishedAt            time.Time        `json:"finished_at"`
	DurationMS            float64          `json:"duration_ms"`
	Summary               Summary          `json:"summary"`
	Thresholds            []CheckResult    `json:"thresholds,omitempty"`
	Invariants            []CheckResult    `json:"invariants,omitempty"`
	Failures              []string         `json:"failures,omitempty"`
	Responses             []ResponseRecord `json:"responses,omitempty"`
}

func Build(runID string, scenarioName string, startedAt time.Time, records []ResponseRecord, thresholds []CheckResult, invariants []CheckResult) Report {
	finishedAt := time.Now().UTC()
	r := Report{
		RunID:      runID,
		Scenario:   scenarioName,
		Status:     StatusPassed,
		StartedAt:  startedAt,
		FinishedAt: finishedAt,
		DurationMS: float64(finishedAt.Sub(startedAt).Microseconds()) / 1000,
		Summary:    Summarize(records),
		Thresholds: thresholds,
		Invariants: invariants,
		Responses:  records,
	}
	for _, check := range append(thresholds, invariants...) {
		if !check.Passed {
			r.Status = StatusFailed
			r.Failures = append(r.Failures, fmt.Sprintf("%s: %s", check.Name, check.Message))
		}
	}
	for _, record := range records {
		if record.Error != "" {
			r.Status = StatusFailed
			break
		}
	}
	return r
}

func Summarize(records []ResponseRecord) Summary {
	s := Summary{StatusCodes: map[string]int{}}
	if len(records) == 0 {
		return s
	}

	var durations []float64
	var total float64
	for _, record := range records {
		s.TotalRequests++
		if record.Error != "" {
			s.FailedRequests++
		}
		statusKey := "error"
		if record.StatusCode > 0 {
			statusKey = fmt.Sprintf("%d", record.StatusCode)
		}
		s.StatusCodes[statusKey]++
		durations = append(durations, record.DurationMS)
		total += record.DurationMS
	}

	s.ErrorRate = float64(s.FailedRequests) / float64(s.TotalRequests)
	sort.Float64s(durations)
	s.Latency = LatencyStats{
		MinMS: durations[0],
		AvgMS: total / float64(len(durations)),
		P50MS: percentile(durations, 0.50),
		P95MS: percentile(durations, 0.95),
		P99MS: percentile(durations, 0.99),
		MaxMS: durations[len(durations)-1],
	}
	return s
}

func (r Report) Text() string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s\n\n", strings.ToUpper(string(r.Status)))
	fmt.Fprintf(&b, "Scenario: %s\n", r.Scenario)
	fmt.Fprintf(&b, "Run ID:   %s\n", r.RunID)
	fmt.Fprintf(&b, "Duration: %.1fms\n\n", r.DurationMS)

	fmt.Fprintf(&b, "Technical summary:\n")
	fmt.Fprintf(&b, "- Requests: %d\n", r.Summary.TotalRequests)
	fmt.Fprintf(&b, "- Failed:   %d\n", r.Summary.FailedRequests)
	fmt.Fprintf(&b, "- Errors:   %.2f%%\n", r.Summary.ErrorRate*100)
	fmt.Fprintf(&b, "- p95:      %.1fms\n", r.Summary.Latency.P95MS)
	fmt.Fprintf(&b, "- Status:   %s\n", formatStatusCodes(r.Summary.StatusCodes))

	if len(r.Failures) > 0 {
		fmt.Fprintf(&b, "\nFailures:\n")
		for _, failure := range r.Failures {
			fmt.Fprintf(&b, "- %s\n", failure)
		}
	}
	return b.String()
}

func (r Report) JSON() ([]byte, error) {
	return json.MarshalIndent(r, "", "  ")
}

func percentile(sorted []float64, p float64) float64 {
	if len(sorted) == 0 {
		return 0
	}
	if len(sorted) == 1 {
		return sorted[0]
	}
	pos := p * float64(len(sorted)-1)
	lower := int(math.Floor(pos))
	upper := int(math.Ceil(pos))
	if lower == upper {
		return sorted[lower]
	}
	weight := pos - float64(lower)
	return sorted[lower]*(1-weight) + sorted[upper]*weight
}

func formatStatusCodes(codes map[string]int) string {
	if len(codes) == 0 {
		return "none"
	}
	keys := make([]string, 0, len(codes))
	for key := range codes {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		parts = append(parts, fmt.Sprintf("%s=%d", key, codes[key]))
	}
	return strings.Join(parts, ", ")
}
