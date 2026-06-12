package main

import (
	"log"

	"github.com/hibiken/asynq"

	"github.com/wreckr/wreckr/apps/api/internal/app"
	"github.com/wreckr/wreckr/apps/api/internal/config"
	"github.com/wreckr/wreckr/apps/api/internal/runexec"
	"github.com/wreckr/wreckr/apps/api/internal/runner"
	"github.com/wreckr/wreckr/apps/api/internal/runqueue"
	"github.com/wreckr/wreckr/apps/api/internal/worker"
)

func main() {
	cfg := config.FromEnv()
	st, err := app.OpenStore(cfg)
	if err != nil {
		log.Fatal(err)
	}

	concurrency := cfg.WorkerConcurrency
	if concurrency <= 0 {
		concurrency = 1
	}
	server := asynq.NewServer(
		asynq.RedisClientOpt{Addr: cfg.RedisAddr},
		asynq.Config{
			Concurrency: concurrency,
			Queues: map[string]int{
				runqueue.QueueRuns: 1,
			},
		},
	)

	mux := asynq.NewServeMux()
	worker.Handler{
		Executor: runexec.Executor{
			Store:   st,
			Runner:  runner.New(),
			Timeout: cfg.RunTimeout,
		},
	}.Register(mux)

	log.Printf("wreckr worker listening on redis %s with concurrency %d", cfg.RedisAddr, concurrency)
	if err := server.Run(mux); err != nil {
		log.Fatal(err)
	}
}
