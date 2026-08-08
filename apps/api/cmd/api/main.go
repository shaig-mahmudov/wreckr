package main

import (
	"context"
	"log"
	"net/http"

	"github.com/wreckr/wreckr/apps/api/internal/app"
	"github.com/wreckr/wreckr/apps/api/internal/config"
	"github.com/wreckr/wreckr/apps/api/internal/httpapi"
	"github.com/wreckr/wreckr/apps/api/internal/runner"
	"github.com/wreckr/wreckr/apps/api/internal/runqueue"
	"github.com/wreckr/wreckr/apps/api/internal/telemetry"
)

func main() {
	cfg := config.FromEnv()
	ctx := context.Background()

	tp, err := telemetry.InitTracer(ctx, "wreckr-api")
	if err != nil {
		log.Printf("failed to initialize tracer: %v", err)
	} else {
		defer func() {
			if err := tp.Shutdown(ctx); err != nil {
				log.Printf("Error shutting down tracer provider: %v", err)
			}
		}()
	}
	st, err := app.OpenStore(cfg)
	if err != nil {
		log.Fatal(err)
	}
	blobStore, err := app.OpenBlobStore(context.Background(), cfg)
	if err != nil {
		log.Fatal(err)
	}
	queue := runqueue.NewAsynqEnqueuer(cfg.RedisAddr, cfg.RunTimeout)
	defer queue.Close()

	server := httpapi.NewWithQueue(cfg, st, blobStore, runner.New(), queue)

	log.Printf("wreckr api listening on %s", cfg.Addr)
	if err := http.ListenAndServe(cfg.Addr, server.Handler()); err != nil {
		log.Fatal(err)
	}
}
