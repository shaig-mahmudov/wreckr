package runqueue

import (
	"context"
	"encoding/json"
	"time"

	"github.com/hibiken/asynq"
)

const (
	QueueRuns      = "runs"
	TypeRunExecute = "runs.execute"
)

type Enqueuer interface {
	EnqueueRun(ctx context.Context, runID string) error
}

type RunPayload struct {
	RunID string `json:"run_id"`
}

type AsynqEnqueuer struct {
	client *asynq.Client
	opts   []asynq.Option
}

func NewAsynqEnqueuer(redisAddr string, timeout time.Duration) *AsynqEnqueuer {
	if timeout <= 0 {
		timeout = 5 * time.Minute
	}
	return &AsynqEnqueuer{
		client: asynq.NewClient(asynq.RedisClientOpt{Addr: redisAddr}),
		opts: []asynq.Option{
			asynq.Queue(QueueRuns),
			asynq.MaxRetry(3),
			asynq.Timeout(timeout),
		},
	}
}

func (e *AsynqEnqueuer) EnqueueRun(ctx context.Context, runID string) error {
	task, err := NewRunTask(runID)
	if err != nil {
		return err
	}
	_, err = e.client.EnqueueContext(ctx, task, e.opts...)
	return err
}

func (e *AsynqEnqueuer) Close() error {
	return e.client.Close()
}

func NewRunTask(runID string) (*asynq.Task, error) {
	raw, err := json.Marshal(RunPayload{RunID: runID})
	if err != nil {
		return nil, err
	}
	return asynq.NewTask(TypeRunExecute, raw), nil
}

func DecodeRunPayload(task *asynq.Task) (RunPayload, error) {
	var payload RunPayload
	err := json.Unmarshal(task.Payload(), &payload)
	return payload, err
}
