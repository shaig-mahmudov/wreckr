package worker

import (
	"context"
	"fmt"

	"github.com/hibiken/asynq"

	"github.com/wreckr/wreckr/apps/api/internal/runexec"
	"github.com/wreckr/wreckr/apps/api/internal/runqueue"
)

type Handler struct {
	Executor runexec.Executor
}

func (h Handler) Register(mux *asynq.ServeMux) {
	mux.HandleFunc(runqueue.TypeRunExecute, h.handleRunExecute)
}

func (h Handler) handleRunExecute(ctx context.Context, task *asynq.Task) error {
	payload, err := runqueue.DecodeRunPayload(task)
	if err != nil {
		return fmt.Errorf("decode run task: %w", err)
	}
	if payload.RunID == "" {
		return fmt.Errorf("run task missing run_id")
	}
	return h.Executor.Execute(ctx, payload.RunID)
}
