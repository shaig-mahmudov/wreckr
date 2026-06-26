package runner

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/wreckr/wreckr/apps/api/internal/report"
	"github.com/wreckr/wreckr/apps/api/internal/runevent"
	"github.com/wreckr/wreckr/apps/api/internal/scenario"
)

const maxBodyPreviewBytes = 4096

type ScenarioRunner interface {
	RunWithOptions(ctx context.Context, sc scenario.Scenario, opts RunOptions) (report.Report, error)
}

type Runner struct {
	Client *http.Client
}

type RunOptions struct {
	RunID  string
	Events runevent.Recorder
}

func New() *Runner {
	return &Runner{
		Client: &http.Client{Timeout: 30 * time.Second},
	}
}

func (r *Runner) Run(ctx context.Context, sc scenario.Scenario) (report.Report, error) {
	return r.RunWithOptions(ctx, sc, RunOptions{})
}

func (r *Runner) RunWithOptions(ctx context.Context, sc scenario.Scenario, opts RunOptions) (report.Report, error) {
	sc = sc.WithEnv().Normalized()
	if err := sc.Validate(); err != nil {
		return report.Report{}, err
	}

	startedAt := time.Now().UTC()
	runID := opts.RunID
	if runID == "" {
		runID = newRunID()
	}
	collector := &recordCollector{}
	limiter := newRequestLimiter(sc.Traffic.RatePerSecond)
	defer limiter.stop()

	setupRecords, setupChecks := r.runControlRequests(ctx, runID, sc, "setup", sc.Setup, limiter, opts.Events)
	if hasFailedCheck(setupChecks) {
		return report.Build(runID, sc.Name, startedAt, setupRecords, setupChecks, nil), nil
	}

	switch sc.Traffic.Type {
	case scenario.TrafficRace:
		r.runRace(ctx, runID, sc, collector, limiter, opts.Events)
	case scenario.TrafficRetryStorm:
		r.runParallel(ctx, runID, sc, collector, max(1, sc.Traffic.Retry.Attempts), limiter, opts.Events)
	default:
		r.runParallel(ctx, runID, sc, collector, 1, limiter, opts.Events)
	}

	records := collector.records()
	thresholds := evaluateThresholds(sc, records)
	emitFailedChecks(runID, opts.Events, thresholds, runevent.TypeThresholdFailed)
	invariants := r.evaluateInvariants(ctx, runID, sc, records, limiter, opts.Events)
	emitFailedChecks(runID, opts.Events, invariants, runevent.TypeInvariantFailed)

	teardownRecords, teardownChecks := r.runControlRequests(ctx, runID, sc, "teardown", sc.Teardown, limiter, opts.Events)
	if hasFailedCheck(teardownChecks) {
		records = append(records, teardownRecords...)
		thresholds = append(thresholds, teardownChecks...)
	}
	return report.Build(runID, sc.Name, startedAt, records, thresholds, invariants), nil
}

func (r *Runner) runParallel(ctx context.Context, runID string, sc scenario.Scenario, collector *recordCollector, attempts int, limiter *requestLimiter, events runevent.Recorder) {
	jobs := make(chan int)
	var wg sync.WaitGroup
	for worker := 0; worker < sc.Traffic.Concurrency; worker++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for iteration := range jobs {
				if ctx.Err() != nil {
					return
				}
				for _, req := range sc.Requests {
					for attempt := 1; attempt <= attempts; attempt++ {
						if ctx.Err() != nil {
							return
						}
						if !limiter.wait(ctx) {
							return
						}
						record := r.executeRequest(ctx, runID, sc, req, iteration, attempt, events)
						collector.add(record)
						if sc.Traffic.Type != scenario.TrafficRetryStorm || sc.Traffic.Retry.BackoffMS <= 0 || attempt == attempts {
							continue
						}
						sleepContext(ctx, time.Duration(sc.Traffic.Retry.BackoffMS)*time.Millisecond)
						if ctx.Err() != nil {
							return
						}
					}
				}
			}
		}()
	}

	for iteration := 0; iteration < sc.Traffic.Iterations; iteration++ {
		select {
		case <-ctx.Done():
			close(jobs)
			wg.Wait()
			return
		case jobs <- iteration:
		}
	}
	close(jobs)
	wg.Wait()
}

func (r *Runner) runRace(ctx context.Context, runID string, sc scenario.Scenario, collector *recordCollector, limiter *requestLimiter, events runevent.Recorder) {
	for iteration := 0; iteration < sc.Traffic.Iterations; iteration++ {
		if ctx.Err() != nil {
			return
		}
		for _, req := range sc.Requests {
			if ctx.Err() != nil {
				return
			}
			startLine := make(chan struct{})
			var wg sync.WaitGroup
			for racer := 0; racer < sc.Traffic.Concurrency; racer++ {
				wg.Add(1)
				go func(racer int, req scenario.Request) {
					defer wg.Done()
					select {
					case <-ctx.Done():
						return
					case <-startLine:
					}
					if ctx.Err() != nil {
						return
					}
					if !limiter.wait(ctx) {
						return
					}
					record := r.executeRequest(ctx, runID, sc, req, iteration, racer+1, events)
					collector.add(record)
				}(racer, req)
			}
			close(startLine)
			wg.Wait()
		}
	}
}

func (r *Runner) executeRequest(ctx context.Context, runID string, sc scenario.Scenario, req scenario.Request, iteration int, attempt int, events runevent.Recorder) report.ResponseRecord {
	startedAt := time.Now().UTC()
	record := report.ResponseRecord{
		RequestName: req.Name,
		Iteration:   iteration,
		Attempt:     attempt,
		StartedAt:   startedAt,
	}
	requestMeta := map[string]any{
		"request":   req.Name,
		"method":    req.Method,
		"path":      req.Path,
		"iteration": iteration,
		"attempt":   attempt,
	}
	emitEvent(runID, events, runevent.LevelInfo, runevent.TypeRequestStarted, "request started", requestMeta)
	defer func() {
		metadata := map[string]any{
			"request":     req.Name,
			"method":      req.Method,
			"path":        req.Path,
			"iteration":   iteration,
			"attempt":     attempt,
			"status_code": record.StatusCode,
			"duration_ms": record.DurationMS,
		}
		if record.Error != "" {
			metadata["error"] = record.Error
		}
		emitEvent(runID, events, runevent.LevelInfo, runevent.TypeRequestCompleted, "request completed", metadata)
	}()

	targetURL, err := joinURL(sc.Target.BaseURL, req.Path)
	if err != nil {
		record.Error = err.Error()
		record.DurationMS = elapsedMS(startedAt)
		return record
	}

	body, contentType := requestBody(req)
	httpReq, err := http.NewRequestWithContext(ctx, req.Method, targetURL, body)
	if err != nil {
		record.Error = err.Error()
		record.DurationMS = elapsedMS(startedAt)
		return record
	}
	for key, value := range sc.Target.Headers {
		httpReq.Header.Set(key, value)
	}
	for key, value := range req.Headers {
		httpReq.Header.Set(key, value)
	}
	if contentType != "" && httpReq.Header.Get("Content-Type") == "" {
		httpReq.Header.Set("Content-Type", contentType)
	}
	httpReq.Header.Set("X-Wreckr-Run-ID", runID)
	httpReq.Header.Set("X-Wreckr-Scenario", sc.Name)
	httpReq.Header.Set("X-Wreckr-Request", req.Name)
	httpReq.Header.Set("X-Wreckr-Iteration", strconv.Itoa(iteration))
	httpReq.Header.Set("X-Wreckr-Attempt", strconv.Itoa(attempt))

	resp, err := r.Client.Do(httpReq)
	record.DurationMS = elapsedMS(startedAt)
	if err != nil {
		record.Error = err.Error()
		return record
	}
	defer resp.Body.Close()

	record.StatusCode = resp.StatusCode
	raw, readErr := io.ReadAll(io.LimitReader(resp.Body, maxBodyPreviewBytes+1))
	if len(raw) > maxBodyPreviewBytes {
		raw = raw[:maxBodyPreviewBytes]
	}
	record.BodyPreview = string(raw)
	if readErr != nil {
		record.Error = readErr.Error()
		return record
	}

	if len(req.Expect.Status) > 0 {
		if !containsStatus(req.Expect.Status, resp.StatusCode) {
			record.Error = fmt.Sprintf("unexpected status %d, expected %v", resp.StatusCode, req.Expect.Status)
			emitEvent(runID, events, runevent.LevelError, runevent.TypeAssertionFailed, record.Error, map[string]any{
				"request":     req.Name,
				"iteration":   iteration,
				"attempt":     attempt,
				"expected":    req.Expect.Status,
				"actual":      resp.StatusCode,
				"status_code": resp.StatusCode,
			})
		}
		return record
	}
	if resp.StatusCode >= 500 {
		record.Error = fmt.Sprintf("server error status %d", resp.StatusCode)
	}
	return record
}

func evaluateThresholds(sc scenario.Scenario, records []report.ResponseRecord) []report.CheckResult {
	var results []report.CheckResult
	summary := report.Summarize(records)

	if sc.Thresholds.MaxErrorRate != nil {
		limit := *sc.Thresholds.MaxErrorRate
		passed := summary.ErrorRate <= limit
		results = append(results, report.CheckResult{
			Name:     "max_error_rate",
			Passed:   passed,
			Message:  comparisonMessage(passed, summary.ErrorRate, "<=", limit),
			Expected: fmt.Sprintf("<= %.4f", limit),
			Actual:   fmt.Sprintf("%.4f", summary.ErrorRate),
		})
	}

	if sc.Thresholds.P95MS != nil {
		limit := *sc.Thresholds.P95MS
		passed := summary.Latency.P95MS <= limit
		results = append(results, report.CheckResult{
			Name:     "p95_ms",
			Passed:   passed,
			Message:  comparisonMessage(passed, summary.Latency.P95MS, "<=", limit),
			Expected: fmt.Sprintf("<= %.1fms", limit),
			Actual:   fmt.Sprintf("%.1fms", summary.Latency.P95MS),
		})
	}

	return results
}

func (r *Runner) evaluateInvariants(ctx context.Context, runID string, sc scenario.Scenario, records []report.ResponseRecord, limiter *requestLimiter, events runevent.Recorder) []report.CheckResult {
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

func (r *Runner) runControlRequests(ctx context.Context, runID string, sc scenario.Scenario, phase string, requests []scenario.Request, limiter *requestLimiter, events runevent.Recorder) ([]report.ResponseRecord, []report.CheckResult) {
	if len(requests) == 0 {
		return nil, nil
	}
	startType := runevent.TypeSetupStarted
	completedType := runevent.TypeSetupCompleted
	if phase == "teardown" {
		startType = runevent.TypeTeardownStarted
		completedType = runevent.TypeTeardownCompleted
	}
	emitEvent(runID, events, runevent.LevelInfo, startType, phase+" started", map[string]any{"requests": len(requests)})
	records := make([]report.ResponseRecord, 0, len(requests))
	checks := make([]report.CheckResult, 0, len(requests))
	defer func() {
		passed := !hasFailedCheck(checks)
		level := runevent.LevelInfo
		if !passed {
			level = runevent.LevelError
		}
		emitEvent(runID, events, level, completedType, phase+" completed", map[string]any{
			"requests": len(records),
			"passed":   passed,
		})
	}()
	for _, req := range requests {
		if ctx.Err() != nil {
			return records, checks
		}
		if !limiter.wait(ctx) {
			return records, checks
		}
		record := r.executeRequest(ctx, runID, sc, req, -1, 1, events)
		records = append(records, record)
		passed := record.Error == ""
		checks = append(checks, report.CheckResult{
			Name:     phase + ":" + req.Name,
			Passed:   passed,
			Message:  controlMessage(passed, record),
			Expected: "request succeeds",
			Actual:   controlActual(record),
		})
		if !passed {
			break
		}
	}
	return records, checks
}

func evaluateResponseCount(inv scenario.Invariant, records []report.ResponseRecord) report.CheckResult {
	count := 0
	for _, record := range records {
		if inv.Request != "" && record.RequestName != inv.Request {
			continue
		}
		if inv.Status != nil && record.StatusCode != *inv.Status {
			continue
		}
		count++
	}

	passed := true
	var expected []string
	if inv.Equals != nil {
		passed = passed && count == *inv.Equals
		expected = append(expected, fmt.Sprintf("equals %d", *inv.Equals))
	}
	if inv.Min != nil {
		passed = passed && count >= *inv.Min
		expected = append(expected, fmt.Sprintf(">= %d", *inv.Min))
	}
	if inv.Max != nil {
		passed = passed && count <= *inv.Max
		expected = append(expected, fmt.Sprintf("<= %d", *inv.Max))
	}

	return report.CheckResult{
		Name:     inv.Name,
		Passed:   passed,
		Message:  countMessage(passed, count, expected),
		Expected: strings.Join(expected, ", "),
		Actual:   strconv.Itoa(count),
	}
}

func (r *Runner) evaluateHTTPProbe(ctx context.Context, runID string, sc scenario.Scenario, inv scenario.Invariant, limiter *requestLimiter) report.CheckResult {
	targetURL, err := joinURL(sc.Target.BaseURL, inv.Path)
	if err != nil {
		return failedInvariant(inv.Name, err.Error())
	}
	req, err := http.NewRequestWithContext(ctx, inv.Method, targetURL, nil)
	if err != nil {
		return failedInvariant(inv.Name, err.Error())
	}
	for key, value := range sc.Target.Headers {
		req.Header.Set(key, value)
	}
	for key, value := range inv.Headers {
		req.Header.Set(key, value)
	}
	req.Header.Set("X-Wreckr-Run-ID", runID)
	req.Header.Set("X-Wreckr-Scenario", sc.Name)
	req.Header.Set("X-Wreckr-Probe", inv.Name)

	if !limiter.wait(ctx) {
		return failedInvariant(inv.Name, ctx.Err().Error())
	}
	resp, err := r.Client.Do(req)
	if err != nil {
		return failedInvariant(inv.Name, err.Error())
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, maxBodyPreviewBytes+1))
	if err != nil {
		return failedInvariant(inv.Name, err.Error())
	}
	if resp.StatusCode >= 400 {
		return failedInvariant(inv.Name, fmt.Sprintf("probe returned status %d", resp.StatusCode))
	}

	got, ok, err := scenario.LookupJSONPath(raw, inv.Expect.JSONPath)
	if err != nil {
		return failedInvariant(inv.Name, err.Error())
	}
	if !ok {
		return failedInvariant(inv.Name, fmt.Sprintf("json path %s was not found", inv.Expect.JSONPath))
	}

	passed := valuesEqual(got, inv.Expect.Equals)
	return report.CheckResult{
		Name:     inv.Name,
		Passed:   passed,
		Message:  valueMessage(passed, got, inv.Expect.Equals),
		Expected: fmt.Sprintf("%v", inv.Expect.Equals),
		Actual:   fmt.Sprintf("%v", got),
	}
}

type recordCollector struct {
	mu    sync.Mutex
	items []report.ResponseRecord
}

type requestLimiter struct {
	ticker *time.Ticker
}

func newRequestLimiter(ratePerSecond int) *requestLimiter {
	if ratePerSecond <= 0 {
		return &requestLimiter{}
	}
	interval := time.Second / time.Duration(ratePerSecond)
	if interval <= 0 {
		interval = time.Nanosecond
	}
	return &requestLimiter{ticker: time.NewTicker(interval)}
}

func (l *requestLimiter) wait(ctx context.Context) bool {
	if l == nil || l.ticker == nil {
		return ctx.Err() == nil
	}
	select {
	case <-ctx.Done():
		return false
	case <-l.ticker.C:
		return true
	}
}

func (l *requestLimiter) stop() {
	if l != nil && l.ticker != nil {
		l.ticker.Stop()
	}
}

func (c *recordCollector) add(record report.ResponseRecord) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.items = append(c.items, record)
}

func (c *recordCollector) records() []report.ResponseRecord {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]report.ResponseRecord, len(c.items))
	copy(out, c.items)
	return out
}

func requestBody(req scenario.Request) (io.Reader, string) {
	if len(req.JSON) > 0 {
		return bytes.NewReader(req.JSON), "application/json"
	}
	if req.Body != "" {
		return strings.NewReader(req.Body), "text/plain"
	}
	return nil, ""
}

func joinURL(baseURL string, path string) (string, error) {
	if strings.HasPrefix(path, "http://") || strings.HasPrefix(path, "https://") {
		return path, nil
	}
	base, err := url.Parse(baseURL)
	if err != nil {
		return "", err
	}
	rel, err := url.Parse(path)
	if err != nil {
		return "", err
	}
	return base.ResolveReference(rel).String(), nil
}

func containsStatus(statuses []int, status int) bool {
	for _, expected := range statuses {
		if expected == status {
			return true
		}
	}
	return false
}

func elapsedMS(startedAt time.Time) float64 {
	return float64(time.Since(startedAt).Microseconds()) / 1000
}

func sleepContext(ctx context.Context, duration time.Duration) {
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-ctx.Done():
	case <-timer.C:
	}
}

func comparisonMessage(passed bool, actual float64, op string, expected float64) string {
	if passed {
		return fmt.Sprintf("passed: %.4f %s %.4f", actual, op, expected)
	}
	return fmt.Sprintf("failed: %.4f is not %s %.4f", actual, op, expected)
}

func countMessage(passed bool, actual int, expected []string) string {
	if passed {
		return fmt.Sprintf("passed: observed %d", actual)
	}
	return fmt.Sprintf("failed: observed %d, expected %s", actual, strings.Join(expected, ", "))
}

func valueMessage(passed bool, actual any, expected any) string {
	if passed {
		return fmt.Sprintf("passed: observed %v", actual)
	}
	return fmt.Sprintf("failed: observed %v, expected %v", actual, expected)
}

func failedInvariant(name string, message string) report.CheckResult {
	return report.CheckResult{Name: name, Passed: false, Message: message}
}

func emitFailedChecks(runID string, events runevent.Recorder, checks []report.CheckResult, eventType runevent.Type) {
	for _, check := range checks {
		if check.Passed {
			continue
		}
		emitEvent(runID, events, runevent.LevelError, eventType, check.Message, map[string]any{
			"check":    check.Name,
			"expected": check.Expected,
			"actual":   check.Actual,
		})
	}
}

func emitEvent(runID string, events runevent.Recorder, level string, eventType runevent.Type, message string, metadata map[string]any) {
	if events == nil {
		return
	}
	events.Record(runevent.Event{
		RunID:    runID,
		Level:    level,
		Type:     eventType,
		Message:  message,
		Metadata: metadata,
	})
}

func hasFailedCheck(checks []report.CheckResult) bool {
	for _, check := range checks {
		if !check.Passed {
			return true
		}
	}
	return false
}

func controlMessage(passed bool, record report.ResponseRecord) string {
	if passed {
		return fmt.Sprintf("passed with status %d", record.StatusCode)
	}
	if record.Error != "" {
		return record.Error
	}
	return fmt.Sprintf("failed with status %d", record.StatusCode)
}

func controlActual(record report.ResponseRecord) string {
	if record.Error != "" {
		return record.Error
	}
	return strconv.Itoa(record.StatusCode)
}

func valuesEqual(actual any, expected any) bool {
	switch a := actual.(type) {
	case float64:
		switch e := expected.(type) {
		case float64:
			return math.Abs(a-e) < 0.000001
		case int:
			return math.Abs(a-float64(e)) < 0.000001
		case json.Number:
			f, err := e.Float64()
			return err == nil && math.Abs(a-f) < 0.000001
		default:
			return fmt.Sprintf("%v", actual) == fmt.Sprintf("%v", expected)
		}
	default:
		return fmt.Sprintf("%v", actual) == fmt.Sprintf("%v", expected)
	}
}

func newRunID() string {
	return fmt.Sprintf("run_%d", time.Now().UTC().UnixNano())
}
