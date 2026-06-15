package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/wreckr/wreckr/apps/api/internal/config"
	"github.com/wreckr/wreckr/apps/api/internal/guardrails"
	"github.com/wreckr/wreckr/apps/api/internal/report"
	"github.com/wreckr/wreckr/apps/api/internal/runevent"
	"github.com/wreckr/wreckr/apps/api/internal/runner"
	"github.com/wreckr/wreckr/apps/api/internal/runqueue"
	"github.com/wreckr/wreckr/apps/api/internal/scenario"
	"github.com/wreckr/wreckr/apps/api/internal/store"
)

type Server struct {
	cfg    config.Config
	store  store.Store
	runner *runner.Runner
	queue  runqueue.Enqueuer

	cancelMu       sync.Mutex
	runCancels     map[string]context.CancelFunc
	cancelRequests map[string]struct{}
}

func New(cfg config.Config, st store.Store, rn *runner.Runner) *Server {
	return NewWithQueue(cfg, st, rn, nil)
}

func NewWithQueue(cfg config.Config, st store.Store, rn *runner.Runner, queue runqueue.Enqueuer) *Server {
	return &Server{
		cfg:            cfg,
		store:          st,
		runner:         rn,
		queue:          queue,
		runCancels:     map[string]context.CancelFunc{},
		cancelRequests: map[string]struct{}{},
	}
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", s.health)
	mux.HandleFunc("GET /metrics", s.metrics)
	mux.HandleFunc("GET /v1/targets", s.listTargets)
	mux.HandleFunc("POST /v1/targets", s.createTarget)
	mux.HandleFunc("GET /v1/targets/{id}", s.getTarget)
	mux.HandleFunc("PUT /v1/targets/{id}", s.updateTarget)
	mux.HandleFunc("DELETE /v1/targets/{id}", s.deleteTarget)
	mux.HandleFunc("GET /v1/scenarios", s.listScenarios)
	mux.HandleFunc("POST /v1/scenarios", s.createScenario)
	mux.HandleFunc("GET /v1/scenarios/{id}", s.getScenario)
	mux.HandleFunc("PUT /v1/scenarios/{id}", s.updateScenario)
	mux.HandleFunc("GET /v1/scenarios/{id}/versions", s.listScenarioVersions)
	mux.HandleFunc("GET /v1/scenarios/{id}/versions/{version}", s.getScenarioVersion)
	mux.HandleFunc("GET /v1/runs", s.listRuns)
	mux.HandleFunc("POST /v1/runs", s.createRun)
	mux.HandleFunc("POST /v1/runs/{id}/cancel", s.cancelRun)
	mux.HandleFunc("GET /v1/runs/{id}/events", s.getRunEvents)
	mux.HandleFunc("GET /v1/runs/{id}/events/stream", s.streamRunEvents)
	mux.HandleFunc("GET /v1/runs/", s.getRun)
	return withCORS(withJSON(mux))
}

func (s *Server) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":      true,
		"service": "wreckr-api",
		"time":    time.Now().UTC(),
	})
}

func (s *Server) metrics(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/plain; version=0.0.4")
	runs := s.store.ListRuns()
	var active, passed, failed, errored, canceled int
	for _, run := range runs {
		switch run.Status {
		case store.RunRunning, store.RunQueued:
			active++
		case store.RunPassed:
			passed++
		case store.RunFailed:
			failed++
		case store.RunErrored:
			errored++
		case store.RunCanceled:
			canceled++
		}
	}
	fmt.Fprintf(w, "wreckr_api_up 1\n")
	fmt.Fprintf(w, "wreckr_runs_total %d\n", len(runs))
	fmt.Fprintf(w, "wreckr_runs_active %d\n", active)
	fmt.Fprintf(w, "wreckr_runs_passed_total %d\n", passed)
	fmt.Fprintf(w, "wreckr_runs_failed_total %d\n", failed)
	fmt.Fprintf(w, "wreckr_runs_errored_total %d\n", errored)
	fmt.Fprintf(w, "wreckr_runs_canceled_total %d\n", canceled)
}

type targetRequest struct {
	Name        string                  `json:"name"`
	BaseURL     string                  `json:"baseUrl"`
	Environment store.TargetEnvironment `json:"environment"`
	Description string                  `json:"description,omitempty"`
	Headers     map[string]string       `json:"headers,omitempty"`
}

func (s *Server) createTarget(w http.ResponseWriter, r *http.Request) {
	target, ok := s.readTargetRequest(w, r)
	if !ok {
		return
	}
	record := s.store.CreateTarget(target)
	writeJSON(w, http.StatusCreated, record)
}

func (s *Server) listTargets(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"targets": s.store.ListTargets()})
}

func (s *Server) getTarget(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	record, ok := s.store.GetTarget(id)
	if !ok {
		writeError(w, http.StatusNotFound, fmt.Errorf("target %q not found", id))
		return
	}
	writeJSON(w, http.StatusOK, record)
}

func (s *Server) updateTarget(w http.ResponseWriter, r *http.Request) {
	target, ok := s.readTargetRequest(w, r)
	if !ok {
		return
	}
	record, ok := s.store.UpdateTarget(r.PathValue("id"), target)
	if !ok {
		writeError(w, http.StatusNotFound, fmt.Errorf("target %q not found", r.PathValue("id")))
		return
	}
	writeJSON(w, http.StatusOK, record)
}

func (s *Server) deleteTarget(w http.ResponseWriter, r *http.Request) {
	if !s.store.DeleteTarget(r.PathValue("id")) {
		writeError(w, http.StatusNotFound, fmt.Errorf("target %q not found", r.PathValue("id")))
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) readTargetRequest(w http.ResponseWriter, r *http.Request) (store.TargetRecord, bool) {
	var req targetRequest
	if err := readJSON(w, r, s.cfg.MaxBodyBytes, &req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return store.TargetRecord{}, false
	}
	target := store.TargetRecord{
		Name:        strings.TrimSpace(req.Name),
		BaseURL:     strings.TrimSpace(req.BaseURL),
		Environment: req.Environment,
		Description: strings.TrimSpace(req.Description),
		Headers:     req.Headers,
	}
	if target.Environment == "" {
		target.Environment = store.TargetDevelopment
	}
	if err := validateTargetRecord(target, s.cfg.Guardrails); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return store.TargetRecord{}, false
	}
	return target, true
}

func (s *Server) createScenario(w http.ResponseWriter, r *http.Request) {
	var sc scenario.Scenario
	if err := readJSON(w, r, s.cfg.MaxBodyBytes, &sc); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	var err error
	sc, _, err = s.resolveScenarioTarget(sc, "")
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	sc = sc.WithEnv().Normalized()
	if err := sc.Validate(); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if err := guardrails.Validate(sc, s.cfg.Guardrails); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	record := s.store.CreateScenario(sc)
	writeJSON(w, http.StatusCreated, record)
}

func (s *Server) listScenarios(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"scenarios": s.store.ListScenarios()})
}

func (s *Server) getScenario(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	record, ok := s.store.GetScenario(id)
	if !ok {
		writeError(w, http.StatusNotFound, fmt.Errorf("scenario %q not found", id))
		return
	}
	writeJSON(w, http.StatusOK, record)
}

func (s *Server) listScenarioVersions(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	versions := s.store.ListScenarioVersions(id)
	if len(versions) == 0 {
		writeError(w, http.StatusNotFound, fmt.Errorf("scenario %q not found", id))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"versions": versions})
}

func (s *Server) getScenarioVersion(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	versionNumber, err := strconv.Atoi(r.PathValue("version"))
	if err != nil || versionNumber < 1 {
		writeError(w, http.StatusBadRequest, errors.New("scenario version must be a positive integer"))
		return
	}
	version, ok := s.store.GetScenarioVersion(id, versionNumber)
	if !ok {
		writeError(w, http.StatusNotFound, fmt.Errorf("scenario %q version %d not found", id, versionNumber))
		return
	}
	writeJSON(w, http.StatusOK, version)
}

func (s *Server) updateScenario(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusNotFound, fmt.Errorf("scenario %q not found", id))
		return
	}

	var sc scenario.Scenario
	if err := readJSON(w, r, s.cfg.MaxBodyBytes, &sc); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	var err error
	sc, _, err = s.resolveScenarioTarget(sc, "")
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	sc = sc.WithEnv().Normalized()
	if err := sc.Validate(); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if err := guardrails.Validate(sc, s.cfg.Guardrails); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	record, ok := s.store.UpdateScenario(id, sc)
	if !ok {
		writeError(w, http.StatusNotFound, fmt.Errorf("scenario %q not found", id))
		return
	}
	writeJSON(w, http.StatusOK, record)
}

type createRunRequest struct {
	ScenarioID string             `json:"scenario_id,omitempty"`
	TargetID   string             `json:"target_id,omitempty"`
  ScenarioVersionNumber int                `json:"scenario_version_number,omitempty"`
	Scenario   *scenario.Scenario `json:"scenario,omitempty"`
	Sync       bool               `json:"sync,omitempty"`
}

func (s *Server) createRun(w http.ResponseWriter, r *http.Request) {
	var req createRunRequest
	if err := readJSON(w, r, s.cfg.MaxBodyBytes, &req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

var sc scenario.Scenario

scenarioID := strings.TrimSpace(req.ScenarioID)
targetID := strings.TrimSpace(req.TargetID)

var versionRef []store.ScenarioVersionRecord

if req.Scenario != nil {
	sc = *req.Scenario
} else if scenarioID != "" {
	if req.ScenarioVersionNumber > 0 {
		version, ok := s.store.GetScenarioVersion(scenarioID, req.ScenarioVersionNumber)
		if !ok {
			writeError(w, http.StatusNotFound, fmt.Errorf("scenario %q version %d not found", scenarioID, req.ScenarioVersionNumber))
			return
		}

		sc = version.Scenario
		versionRef = append(versionRef, version)
	} else {
		record, ok := s.store.GetScenario(scenarioID)
		if !ok {
			writeError(w, http.StatusNotFound, fmt.Errorf("scenario %q not found", scenarioID))
			return
		}

		sc = record.Scenario
	}
} else {
	writeError(w, http.StatusBadRequest, errors.New("scenario_id or scenario is required"))
	return
}

	var err error
	sc, targetID, err = s.resolveScenarioTarget(sc, targetID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	sc = sc.WithEnv().Normalized()
	if err := sc.Validate(); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if err := guardrails.Validate(sc, s.cfg.Guardrails); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	sc = s.applyRunGuardrails(sc)

  runRecord := s.store.CreateRun(scenarioID, targetID, sc, versionRef...)

  if s.queue != nil {
    if err := s.queue.EnqueueRun(r.Context(), runRecord.ID); err != nil {
      s.store.ErrorRun(runRecord.ID, fmt.Errorf("enqueue run: %w", err))
      writeError(w, http.StatusServiceUnavailable, fmt.Errorf("enqueue run: %w", err))
      return
    }

    writeJSON(w, http.StatusAccepted, runRecord)
    return
  }

	if req.Sync {
		s.executeRun(r.Context(), runRecord.ID, sc)
		updated, _ := s.store.GetRun(runRecord.ID)
		writeJSON(w, http.StatusCreated, updated)
		return
	}

	go s.executeRun(context.Background(), runRecord.ID, sc)
	writeJSON(w, http.StatusAccepted, runRecord)
}

func (s *Server) listRuns(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"runs": s.store.ListRuns()})
}

func (s *Server) getRun(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/v1/runs/")
	id := strings.TrimSuffix(path, "/report")
	record, ok := s.store.GetRun(id)
	if !ok {
		writeError(w, http.StatusNotFound, fmt.Errorf("run %q not found", id))
		return
	}
	if strings.HasSuffix(path, "/report") {
		if record.Report == nil {
			writeError(w, http.StatusConflict, errors.New("report is not ready"))
			return
		}
		writeJSON(w, http.StatusOK, record.Report)
		return
	}
	writeJSON(w, http.StatusOK, record)
}

func (s *Server) cancelRun(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	record, ok := s.store.GetRun(id)
	if !ok {
		writeError(w, http.StatusNotFound, fmt.Errorf("run %q not found", id))
		return
	}
	if !isCancelableRunStatus(record.Status) {
		writeError(w, http.StatusConflict, fmt.Errorf("run %q cannot be canceled from status %q", id, record.Status))
		return
	}
	s.store.AppendRunEvent(id, runevent.Event{
		Level:   runevent.LevelWarn,
		Type:    runevent.TypeCancelRequested,
		Message: "run cancellation requested",
		Metadata: map[string]any{
			"status": string(record.Status),
		},
	})
	if !s.requestRunCancel(id) {
		if s.queue != nil && record.Status == store.RunQueued {
			s.store.CancelRun(id, canceledReport(id, record.Scenario.Name, "run canceled before worker execution"))
			writeJSON(w, http.StatusAccepted, map[string]any{"id": id, "status": store.RunCanceled})
			return
		}
		writeError(w, http.StatusConflict, fmt.Errorf("run %q is not currently cancellable", id))
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]any{"id": id, "status": store.RunCanceled})
}

func (s *Server) getRunEvents(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if _, ok := s.store.GetRun(id); !ok {
		writeError(w, http.StatusNotFound, fmt.Errorf("run %q not found", id))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"events": s.store.ListRunEvents(id)})
}

func (s *Server) streamRunEvents(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if _, ok := s.store.GetRun(id); !ok {
		writeError(w, http.StatusNotFound, fmt.Errorf("run %q not found", id))
		return
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, errors.New("streaming is not supported"))
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	lastSequence := lastEventSequence(r)
	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()

	for {
		events := s.store.ListRunEvents(id)
		for _, event := range events {
			if event.Sequence <= lastSequence {
				continue
			}
			if err := writeSSEEvent(w, event); err != nil {
				return
			}
			lastSequence = event.Sequence
		}
		flusher.Flush()

		record, ok := s.store.GetRun(id)
		if !ok || isTerminalRunStatus(record.Status) || hasTerminalEvent(events) {
			return
		}

		select {
		case <-r.Context().Done():
			return
		case <-ticker.C:
		}
	}
}

func (s *Server) executeRun(parent context.Context, id string, sc scenario.Scenario) {
	ctx, cancel := context.WithTimeout(parent, s.effectiveRunTimeout())
	s.registerRunCancel(id, cancel)
	defer cancel()

	s.store.MarkRunStarted(id)
	rep, err := s.runner.RunWithOptions(ctx, sc, runner.RunOptions{
		RunID: id,
		Events: runevent.RecorderFunc(func(event runevent.Event) {
			s.store.AppendRunEvent(id, event)
		}),
	})
	if s.finishRunCancel(id) {
		if err != nil {
			rep = reportFromRunError(id, sc.Name, err)
		}
		rep.Failures = append(rep.Failures, "run canceled by user")
		s.store.CancelRun(id, rep)
		return
	}
	if err != nil {
		s.store.ErrorRun(id, err)
		return
	}
	s.store.CompleteRun(id, rep)
}

func (s *Server) effectiveRunTimeout() time.Duration {
	if s.cfg.Guardrails.MaxRunDuration > 0 && s.cfg.Guardrails.MaxRunDuration < s.cfg.RunTimeout {
		return s.cfg.Guardrails.MaxRunDuration
	}
	return s.cfg.RunTimeout
}

func (s *Server) applyRunGuardrails(sc scenario.Scenario) scenario.Scenario {
	if s.cfg.Guardrails.MaxRequestRate > 0 && sc.Traffic.RatePerSecond == 0 {
		sc.Traffic.RatePerSecond = s.cfg.Guardrails.MaxRequestRate
	}
	return sc
}

func (s *Server) resolveScenarioTarget(sc scenario.Scenario, explicitTargetID string) (scenario.Scenario, string, error) {
	targetID := strings.TrimSpace(explicitTargetID)
	if targetID == "" {
		targetID = strings.TrimSpace(sc.Target.ID)
	}
	if targetID == "" {
		return sc, "", nil
	}
	target, ok := s.store.GetTarget(targetID)
	if !ok {
		return scenario.Scenario{}, "", fmt.Errorf("target %q not found", targetID)
	}
	sc.Target.ID = target.ID
	sc.Target.BaseURL = target.BaseURL
	sc.Target.Headers = mergeHeaders(target.Headers, sc.Target.Headers)
	return sc, target.ID, nil
}

func validateTargetRecord(target store.TargetRecord, cfg config.Guardrails) error {
	var problems []string
	if target.Name == "" {
		problems = append(problems, "name is required")
	}
	switch target.Environment {
	case store.TargetLocal, store.TargetDevelopment, store.TargetStaging, store.TargetProduction:
	default:
		problems = append(problems, "environment must be one of local, development, staging, production")
	}
	if err := guardrails.ValidateTargetBaseURL(target.BaseURL, cfg); err != nil {
		problems = append(problems, err.Error())
	}
	for key := range target.Headers {
		if strings.TrimSpace(key) == "" {
			problems = append(problems, "header names must not be empty")
			break
		}
	}
	if len(problems) > 0 {
		return errors.New(strings.Join(problems, "; "))
	}
	return nil
}

func mergeHeaders(targetHeaders map[string]string, scenarioHeaders map[string]string) map[string]string {
	if len(targetHeaders) == 0 && len(scenarioHeaders) == 0 {
		return map[string]string{}
	}
	merged := make(map[string]string, len(targetHeaders)+len(scenarioHeaders))
	for key, value := range targetHeaders {
		merged[key] = value
	}
	for key, value := range scenarioHeaders {
		merged[key] = value
	}
	return merged
}

func (s *Server) registerRunCancel(id string, cancel context.CancelFunc) {
	s.cancelMu.Lock()
	defer s.cancelMu.Unlock()
	s.runCancels[id] = cancel
}

func (s *Server) requestRunCancel(id string) bool {
	s.cancelMu.Lock()
	cancel, ok := s.runCancels[id]
	if ok {
		s.cancelRequests[id] = struct{}{}
	}
	s.cancelMu.Unlock()

	if ok {
		cancel()
	}
	return ok
}

func (s *Server) finishRunCancel(id string) bool {
	s.cancelMu.Lock()
	defer s.cancelMu.Unlock()
	_, canceled := s.cancelRequests[id]
	delete(s.cancelRequests, id)
	delete(s.runCancels, id)
	return canceled
}

func isCancelableRunStatus(status store.RunStatus) bool {
	return status == store.RunQueued || status == store.RunRunning
}

func isTerminalRunStatus(status store.RunStatus) bool {
	switch status {
	case store.RunPassed, store.RunFailed, store.RunErrored, store.RunCanceled:
		return true
	default:
		return false
	}
}

func hasTerminalEvent(events []runevent.Event) bool {
	for _, event := range events {
		switch event.Type {
		case runevent.TypeRunCompleted, runevent.TypeRunFailed, runevent.TypeRunCanceled:
			return true
		}
	}
	return false
}

func lastEventSequence(r *http.Request) int64 {
	value := r.Header.Get("Last-Event-ID")
	if value == "" {
		value = r.URL.Query().Get("after")
	}
	sequence, err := strconv.ParseInt(value, 10, 64)
	if err != nil || sequence < 0 {
		return 0
	}
	return sequence
}

func writeSSEEvent(w http.ResponseWriter, event runevent.Event) error {
	raw, err := json.Marshal(event)
	if err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "id: %d\n", event.Sequence); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "event: %s\n", event.Type); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "data: %s\n\n", raw); err != nil {
		return err
	}
	return nil
}

func reportFromRunError(id string, scenarioName string, err error) report.Report {
	return canceledReport(id, scenarioName, err.Error())
}

func canceledReport(id string, scenarioName string, message string) report.Report {
	now := time.Now().UTC()
	return report.Report{
		RunID:      id,
		Scenario:   scenarioName,
		Status:     report.StatusCanceled,
		StartedAt:  now,
		FinishedAt: now,
		Failures:   []string{message},
	}
}

func readJSON(w http.ResponseWriter, r *http.Request, maxBytes int64, dst any) error {
	r.Body = http.MaxBytesReader(w, r.Body, maxBytes)
	defer r.Body.Close()
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		return err
	}
	return nil
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeError(w http.ResponseWriter, status int, err error) {
	writeJSON(w, status, map[string]any{"error": err.Error()})
}

func withJSON(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		next.ServeHTTP(w, r)
	})
}

func withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET,POST,PUT,DELETE,OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type,Authorization")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}
