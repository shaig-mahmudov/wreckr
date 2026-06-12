package runexec

import (
	"context"
	"fmt"
	"time"

	"github.com/wreckr/wreckr/apps/api/internal/runevent"
	"github.com/wreckr/wreckr/apps/api/internal/runner"
	"github.com/wreckr/wreckr/apps/api/internal/store"
)

type Executor struct {
	Store   store.Store
	Runner  *runner.Runner
	Timeout time.Duration
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

	if record.Status == store.RunQueued {
		e.Store.MarkRunStarted(runID)
	}
	rep, err := e.Runner.RunWithOptions(runCtx, record.Scenario, runner.RunOptions{
		RunID: runID,
		Events: runevent.RecorderFunc(func(event runevent.Event) {
			e.Store.AppendRunEvent(runID, event)
		}),
	})
	if err != nil {
		e.Store.ErrorRun(runID, err)
		return err
	}
	e.Store.CompleteRun(runID, rep)
	return nil
}
