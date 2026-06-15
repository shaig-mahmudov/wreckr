package runexec

import (
	"context"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/wreckr/wreckr/apps/api/internal/report"
	"github.com/wreckr/wreckr/apps/api/internal/runevent"
	"github.com/wreckr/wreckr/apps/api/internal/runner"
	"github.com/wreckr/wreckr/apps/api/internal/store"
)

type Executor struct {
	Store               store.Store
	Runner              *runner.Runner
	Timeout             time.Duration
	CancelCheckInterval time.Duration
}

func (e Executor) Execute(ctx context.Context, runID string) error {
	record, ok := e.Store.GetRun(runID)
	if !ok {
		return fmt.Errorf("run %q not found", runID)
	}
	switch record.Status {
	case store.RunQueued, store.RunRunning:
	case store.RunPassed, store.RunFailed, store.RunErrored, store.RunCanceled:
		return nil
	default:
		return fmt.Errorf("run %q cannot execute from status %q", runID, record.Status)
	}

	timeout := e.Timeout
	if timeout <= 0 {
		timeout = 5 * time.Minute
	}
	runCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	if e.Store.IsRunCancelRequested(runID) {
		e.Store.CancelRun(runID, canceledReport(runID, record.Scenario.Name, "run canceled before worker execution"))
		return nil
	}

	if record.Status == store.RunQueued {
		e.Store.MarkRunStarted(runID)
	}
	started, ok := e.Store.GetRun(runID)
	if !ok || started.Status != store.RunRunning {
		return nil
	}

	var cancelRequested atomic.Bool
	stopMonitor := make(chan struct{})
	defer close(stopMonitor)
	go e.monitorCancellation(runCtx, stopMonitor, runID, cancel, &cancelRequested)

	rep, err := e.Runner.RunWithOptions(runCtx, record.Scenario, runner.RunOptions{
		RunID: runID,
		Events: runevent.RecorderFunc(func(event runevent.Event) {
			e.Store.AppendRunEvent(runID, event)
		}),
	})
	if cancelRequested.Load() || e.Store.IsRunCancelRequested(runID) {
		if err != nil {
			rep = canceledReport(runID, record.Scenario.Name, err.Error())
		}
		rep.Failures = appendIfMissing(rep.Failures, "run canceled by user")
		e.Store.CancelRun(runID, rep)
		return nil
	}
	if err != nil {
		e.Store.ErrorRun(runID, err)
		return err
	}
	e.Store.CompleteRun(runID, rep)
	return nil
}

func (e Executor) monitorCancellation(ctx context.Context, stop <-chan struct{}, runID string, cancel context.CancelFunc, requested *atomic.Bool) {
	interval := e.CancelCheckInterval
	if interval <= 0 {
		interval = 100 * time.Millisecond
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-stop:
			return
		case <-ticker.C:
			if e.Store.IsRunCancelRequested(runID) {
				requested.Store(true)
				cancel()
				return
			}
		}
	}
}

func canceledReport(id string, scenarioName string, message string) report.Report {
	now := time.Now().UTC()
	failures := []string{}
	if message != "" {
		failures = append(failures, message)
	}
	return report.Report{
		RunID:      id,
		Scenario:   scenarioName,
		Status:     report.StatusCanceled,
		StartedAt:  now,
		FinishedAt: now,
		Failures:   failures,
	}
}

func appendIfMissing(values []string, value string) []string {
	for _, existing := range values {
		if existing == value {
			return values
		}
	}
	return append(values, value)
}
