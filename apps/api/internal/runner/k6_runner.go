package runner

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/wreckr/wreckr/apps/api/internal/k6script"
	"github.com/wreckr/wreckr/apps/api/internal/report"
	"github.com/wreckr/wreckr/apps/api/internal/scenario"
)

type K6Runner struct {
	Client *http.Client
}

func NewK6Runner() *K6Runner {
	return &K6Runner{
		Client: &http.Client{Timeout: 30 * time.Second},
	}
}

// Ensure K6Runner implements ScenarioRunner
var _ ScenarioRunner = (*K6Runner)(nil)

type k6Summary struct {
	Metrics   map[string]k6Metric `json:"metrics"`
	RootGroup k6Group             `json:"root_group"`
}

type k6Metric struct {
	Type       string                 `json:"type"`
	Values     map[string]float64     `json:"values"`
	Thresholds map[string]k6Threshold `json:"thresholds,omitempty"`
}

type k6Threshold struct {
	Ok bool `json:"ok"`
}

type k6Group struct {
	Name   string    `json:"name"`
	Path   string    `json:"path"`
	Checks []k6Check `json:"checks"`
	Groups []k6Group `json:"groups"`
}

type k6Check struct {
	Name   string `json:"name"`
	Path   string `json:"path"`
	Passes int    `json:"passes"`
	Fails  int    `json:"fails"`
}

func (kr *K6Runner) RunWithOptions(ctx context.Context, sc scenario.Scenario, opts RunOptions) (report.Report, error) {
	// Compile to k6 script
	script, err := k6script.Compile(sc)
	if err != nil {
		return report.Report{}, fmt.Errorf("compile scenario: %w", err)
	}

	// Create temp script file
	tempScript, err := os.CreateTemp("", "wreckr-k6-*.js")
	if err != nil {
		return report.Report{}, fmt.Errorf("create temp script: %w", err)
	}
	defer os.Remove(tempScript.Name())
	defer tempScript.Close()

	if _, err := tempScript.WriteString(script.Content); err != nil {
		return report.Report{}, fmt.Errorf("write temp script: %w", err)
	}

	// Create temp summary JSON file path
	tempSummary, err := os.CreateTemp("", "wreckr-k6-summary-*.json")
	if err != nil {
		return report.Report{}, fmt.Errorf("create temp summary file: %w", err)
	}
	tempSummary.Close()
	defer os.Remove(tempSummary.Name())

	startedAt := time.Now().UTC()

	// Execute k6 run
	cmd := exec.CommandContext(ctx, "k6", "run", "--summary-export", tempSummary.Name(), tempScript.Name())
	var stdoutBuf, stderrBuf bytes.Buffer
	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf

	err = cmd.Run()
	// Even if command failed, check if summary was created (e.g. threshold failed)
	summaryData, readErr := os.ReadFile(tempSummary.Name())
	if readErr != nil {
		// If summary doesn't exist, k6 failed to run completely
		return report.Report{}, fmt.Errorf("k6 run failed: %v, stderr: %s", err, stderrBuf.String())
	}

	var summary k6Summary
	if err := json.Unmarshal(summaryData, &summary); err != nil {
		return report.Report{}, fmt.Errorf("unmarshal k6 summary: %w", err)
	}

	// Reconstruct mock response records from k6 status code metrics
	records := kr.reconstructRecords(sc, &summary, startedAt)

	// Run HTTP probe invariants from Go
	var invariants []report.CheckResult
	if len(sc.Invariants) > 0 {
		limiter := newRequestLimiter(sc.Traffic.RatePerSecond)
		defer limiter.stop()

		invariants = kr.evaluateInvariants(ctx, opts.RunID, sc, records, limiter)
	}

	// Reconstruct thresholds from the k6 summary thresholds
	thresholds := kr.parseThresholds(sc, &summary)

	// Build base report
	rep := report.Build(opts.RunID, sc.Name, startedAt, records, thresholds, invariants)

	// Overwrite latency metrics with k6's actual metrics
	if durationMetric, ok := summary.Metrics["http_req_duration"]; ok {
		rep.Summary.Latency = report.LatencyStats{
			MinMS: durationMetric.Values["min"],
			AvgMS: durationMetric.Values["avg"],
			P50MS: durationMetric.Values["med"],
			P95MS: durationMetric.Values["p(95)"],
			P99MS: durationMetric.Values["p(99)"],
			MaxMS: durationMetric.Values["max"],
		}
	}

	return rep, nil
}

// reconstructRecords parses the k6 summary metrics using regex to find status codes per request
func (kr *K6Runner) reconstructRecords(sc scenario.Scenario, summary *k6Summary, startedAt time.Time) []report.ResponseRecord {
	var records []report.ResponseRecord

	// Regex to match "http_reqs{name:<name>,status:<code>}"
	// Quotation marks around request name in selector might be present: name:"checkout"
	re := regexp.MustCompile(`^http_reqs\{name:(?:"([^"]+)"|([^,]+)),status:(\d+)\}$`)

	// Track average duration for mock records
	avgDuration := 0.0
	if durationMetric, ok := summary.Metrics["http_req_duration"]; ok {
		avgDuration = durationMetric.Values["avg"]
	}

	for key, metric := range summary.Metrics {
		matches := re.FindStringSubmatch(key)
		if len(matches) == 0 {
			continue
		}

		reqName := matches[1]
		if reqName == "" {
			reqName = matches[2]
		}
		statusCode, _ := strconv.Atoi(matches[3])
		count := int(metric.Values["count"])

		for i := 0; i < count; i++ {
			rec := report.ResponseRecord{
				RequestName: reqName,
				StatusCode:  statusCode,
				DurationMS:  avgDuration,
				StartedAt:   startedAt,
			}
			if statusCode == 0 || statusCode >= 500 {
				rec.Error = fmt.Sprintf("status code %d", statusCode)
			}
			records = append(records, rec)
		}
	}

	return records
}

func (kr *K6Runner) evaluateInvariants(ctx context.Context, runID string, sc scenario.Scenario, records []report.ResponseRecord, limiter *requestLimiter) []report.CheckResult {
	// Re-use Go runner's HTTP probe invariant evaluation.
	r := &Runner{Client: kr.Client}
	results := make([]report.CheckResult, 0, len(sc.Invariants))
	for _, invariant := range sc.Invariants {
		switch invariant.Type {
		case "response_count":
			results = append(results, evaluateResponseCount(invariant, records))
		case "http_probe":
			results = append(results, r.evaluateHTTPProbe(ctx, runID, sc, invariant, limiter))
		}
	}
	return results
}

func (kr *K6Runner) parseThresholds(sc scenario.Scenario, summary *k6Summary) []report.CheckResult {
	var results []report.CheckResult

	if sc.Thresholds.MaxErrorRate != nil {
		limit := *sc.Thresholds.MaxErrorRate
		successRate := 1.0 - limit

		// Find the checks metric threshold
		passed := true
		actualStr := "passed"

		if checksMetric, ok := summary.Metrics["checks"]; ok {
			for tKey, tVal := range checksMetric.Thresholds {
				if strings.Contains(tKey, fmt.Sprintf("rate>=%.4f", successRate)) {
					passed = tVal.Ok
					if rate, ok := checksMetric.Values["rate"]; ok {
						actualStr = fmt.Sprintf("%.4f", 1.0-rate)
					}
					break
				}
			}
		}

		results = append(results, report.CheckResult{
			Name:     "max_error_rate",
			Passed:   passed,
			Message:  fmt.Sprintf("%s: error rate %s (limit <= %.4f)", statusText(passed), actualStr, limit),
			Expected: fmt.Sprintf("<= %.4f", limit),
			Actual:   actualStr,
		})
	}

	if sc.Thresholds.P95MS != nil {
		limit := *sc.Thresholds.P95MS
		passed := true
		actualStr := "passed"

		if durationMetric, ok := summary.Metrics["http_req_duration"]; ok {
			for tKey, tVal := range durationMetric.Thresholds {
				if strings.Contains(tKey, fmt.Sprintf("p(95)<=%.1f", limit)) {
					passed = tVal.Ok
					if p95, ok := durationMetric.Values["p(95)"]; ok {
						actualStr = fmt.Sprintf("%.1fms", p95)
					}
					break
				}
			}
		}

		results = append(results, report.CheckResult{
			Name:     "p95_ms",
			Passed:   passed,
			Message:  fmt.Sprintf("%s: p95 latency %s (limit <= %.1fms)", statusText(passed), actualStr, limit),
			Expected: fmt.Sprintf("<= %.1fms", limit),
			Actual:   actualStr,
		})
	}

	return results
}

func statusText(passed bool) string {
	if passed {
		return "passed"
	}
	return "failed"
}
