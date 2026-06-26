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
)

func main() {
	cfg := config.FromEnv()
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
