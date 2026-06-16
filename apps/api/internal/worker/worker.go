package worker

import (
	"context"
	"fmt"

	"github.com/hibiken/asynq"

	"github.com/wreckr/wreckr/apps/api/internal/runevent"
	"github.com/wreckr/wreckr/apps/api/internal/runexec"
	"github.com/wreckr/wreckr/apps/api/internal/runqueue"
	"github.com/wreckr/wreckr/apps/api/internal/store"
)

type Executor interface {
	Execute(ctx context.Context, runID string) error
}

type Handler struct {
	Executor Executor
	Events   store.Store
	TaskInfo func(context.Context) TaskInfo
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
	h.recordWorkerEvent(ctx, task, payload.RunID, runevent.Event{
		Level:   runevent.LevelInfo,
		Type:    runevent.TypeWorkerAttemptStarted,
		Message: "worker attempt started",
	})
	return h.Executor.Execute(ctx, payload.RunID)
}

func (h Handler) HandleError(ctx context.Context, task *asynq.Task, err error) {
	payload, decodeErr := runqueue.DecodeRunPayload(task)
	if decodeErr != nil || payload.RunID == "" {
		return
	}

	info := h.taskInfo(ctx)
	h.recordWorkerEvent(ctx, task, payload.RunID, runevent.Event{
		Level:   runevent.LevelError,
		Type:    runevent.TypeWorkerAttemptFailed,
		Message: "worker attempt failed",
		Metadata: map[string]any{
			"error": err.Error(),
		},
	})
	if info.Known && info.RetryCount >= info.MaxRetry {
		h.recordWorkerEvent(ctx, task, payload.RunID, runevent.Event{
			Level:   runevent.LevelError,
			Type:    runevent.TypeWorkerDeadLettered,
			Message: "worker retries exhausted",
			Metadata: map[string]any{
				"error": err.Error(),
			},
		})
		return
	}
	if info.Known {
		h.recordWorkerEvent(ctx, task, payload.RunID, runevent.Event{
			Level:   runevent.LevelWarn,
			Type:    runevent.TypeWorkerRetryScheduled,
			Message: "worker retry scheduled",
			Metadata: map[string]any{
				"error": err.Error(),
			},
		})
	}
}

type TaskInfo struct {
	TaskID     string
	Queue      string
	RetryCount int
	MaxRetry   int
	Known      bool
}

func (h Handler) recordWorkerEvent(ctx context.Context, task *asynq.Task, runID string, event runevent.Event) {
	if h.Events == nil {
		return
	}
	info := h.taskInfo(ctx)
	if event.Metadata == nil {
		event.Metadata = map[string]any{}
	}
	event.Metadata["task_type"] = task.Type()
	if info.TaskID != "" {
		event.Metadata["task_id"] = info.TaskID
	}
	if info.Queue != "" {
		event.Metadata["queue"] = info.Queue
	}
	if info.Known {
		event.Metadata["attempt"] = info.RetryCount + 1
		event.Metadata["retry_count"] = info.RetryCount
		event.Metadata["max_retries"] = info.MaxRetry
	}
	h.Events.AppendRunEvent(runID, event)
}

func (h Handler) taskInfo(ctx context.Context) TaskInfo {
	if h.TaskInfo != nil {
		return h.TaskInfo(ctx)
	}
	return taskInfoFromAsynq(ctx)
}

func taskInfoFromAsynq(ctx context.Context) TaskInfo {
	var info TaskInfo
	if taskID, ok := asynq.GetTaskID(ctx); ok {
		info.TaskID = taskID
	}
	if queue, ok := asynq.GetQueueName(ctx); ok {
		info.Queue = queue
	}
	retryCount, retryOK := asynq.GetRetryCount(ctx)
	maxRetry, maxRetryOK := asynq.GetMaxRetry(ctx)
	if retryOK && maxRetryOK {
		info.RetryCount = retryCount
		info.MaxRetry = maxRetry
		info.Known = true
	}
	return info
}

var _ Executor = runexec.Executor{}
var _ asynq.ErrorHandler = Handler{}
