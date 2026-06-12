package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/wreckr/wreckr/apps/api/internal/config"
	"github.com/wreckr/wreckr/apps/api/internal/report"
	"github.com/wreckr/wreckr/apps/api/internal/runner"
	"github.com/wreckr/wreckr/apps/api/internal/scenario"
	"github.com/wreckr/wreckr/apps/api/internal/store"
)

type Server struct {
	cfg    config.Config
	store  store.Store
	runner *runner.Runner

	cancelMu       sync.Mutex
	runCancels     map[string]context.CancelFunc
	cancelRequests map[string]struct{}
}

func New(cfg config.Config, st store.Store, rn *runner.Runner) *Server {
	return &Server{
		cfg:            cfg,
		store:          st,
		runner:         rn,
		runCancels:     map[string]context.CancelFunc{},
		cancelRequests: map[string]struct{}{},
	}
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", s.health)
	mux.HandleFunc("GET /metrics", s.metrics)
	mux.HandleFunc("GET /v1/scenarios", s.listScenarios)
	mux.HandleFunc("POST /v1/scenarios", s.createScenario)
	mux.HandleFunc("PUT /v1/scenarios/", s.updateScenario)
	mux.HandleFunc("GET /v1/scenarios/", s.getScenario)
	mux.HandleFunc("GET /v1/runs", s.listRuns)
	mux.HandleFunc("POST /v1/runs", s.createRun)
	mux.HandleFunc("POST /v1/runs/{id}/cancel", s.cancelRun)
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

func (s *Server) createScenario(w http.ResponseWriter, r *http.Request) {
	var sc scenario.Scenario
	if err := readJSON(w, r, s.cfg.MaxBodyBytes, &sc); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	sc = sc.WithEnv().Normalized()
	if err := sc.Validate(); err != nil {
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
	id := strings.TrimPrefix(r.URL.Path, "/v1/scenarios/")
	if strings.HasSuffix(id, "/versions") {
		id = strings.TrimSuffix(id, "/versions")
		versions := s.store.ListScenarioVersions(id)
		if len(versions) == 0 {
			writeError(w, http.StatusNotFound, fmt.Errorf("scenario %q not found", id))
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"versions": versions})
		return
	}
	record, ok := s.store.GetScenario(id)
	if !ok {
		writeError(w, http.StatusNotFound, fmt.Errorf("scenario %q not found", id))
		return
	}
	writeJSON(w, http.StatusOK, record)
}

func (s *Server) updateScenario(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/v1/scenarios/")
	if id == "" || strings.Contains(id, "/") {
		writeError(w, http.StatusNotFound, fmt.Errorf("scenario %q not found", id))
		return
	}

	var sc scenario.Scenario
	if err := readJSON(w, r, s.cfg.MaxBodyBytes, &sc); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	sc = sc.WithEnv().Normalized()
	if err := sc.Validate(); err != nil {
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
	scenarioID := req.ScenarioID
	if req.Scenario != nil {
		sc = *req.Scenario
	} else if req.ScenarioID != "" {
		record, ok := s.store.GetScenario(req.ScenarioID)
		if !ok {
			writeError(w, http.StatusNotFound, fmt.Errorf("scenario %q not found", req.ScenarioID))
			return
		}
		sc = record.Scenario
	} else {
		writeError(w, http.StatusBadRequest, errors.New("scenario_id or scenario is required"))
		return
	}

	sc = sc.WithEnv().Normalized()
	if err := sc.Validate(); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	runRecord := s.store.CreateRun(scenarioID, sc)
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
	if !s.requestRunCancel(id) {
		writeError(w, http.StatusConflict, fmt.Errorf("run %q is not currently cancellable", id))
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]any{"id": id, "status": store.RunCanceled})
}

func (s *Server) executeRun(parent context.Context, id string, sc scenario.Scenario) {
	ctx, cancel := context.WithTimeout(parent, s.cfg.RunTimeout)
	s.registerRunCancel(id, cancel)
	defer cancel()

	s.store.MarkRunStarted(id)
	rep, err := s.runner.Run(ctx, sc)
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

func reportFromRunError(id string, scenarioName string, err error) report.Report {
	now := time.Now().UTC()
	return report.Report{
		RunID:      id,
		Scenario:   scenarioName,
		Status:     report.StatusCanceled,
		StartedAt:  now,
		FinishedAt: now,
		Failures:   []string{err.Error()},
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
		w.Header().Set("Access-Control-Allow-Methods", "GET,POST,OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type,Authorization")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}
