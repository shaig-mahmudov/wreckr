package runqueue

import (
	"github.com/hibiken/asynq"
)

type QueueInspector interface {
	ListArchivedTasks(queue string, limit int) ([]*asynq.TaskInfo, error)
	RunTask(queue, id string) error
	DeleteTask(queue, id string) error
	Close() error
}

type AsynqInspector struct {
	inspector *asynq.Inspector
}

func NewAsynqInspector(redisAddr string) *AsynqInspector {
	return &AsynqInspector{
		inspector: asynq.NewInspector(asynq.RedisClientOpt{Addr: redisAddr}),
	}
}

func (i *AsynqInspector) ListArchivedTasks(queue string, limit int) ([]*asynq.TaskInfo, error) {
	return i.inspector.ListArchivedTasks(queue, asynq.PageSize(limit))
}

func (i *AsynqInspector) RunTask(queue, id string) error {
	return i.inspector.RunTask(queue, id)
}

func (i *AsynqInspector) DeleteTask(queue, id string) error {
	return i.inspector.DeleteTask(queue, id)
}

func (i *AsynqInspector) Close() error {
	return i.inspector.Close()
}
