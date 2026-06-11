package main

import (
	"log"
	"net/http"

	"github.com/wreckr/wreckr/apps/api/internal/config"
	"github.com/wreckr/wreckr/apps/api/internal/httpapi"
	"github.com/wreckr/wreckr/apps/api/internal/runner"
	"github.com/wreckr/wreckr/apps/api/internal/store"
)

func main() {
	cfg := config.FromEnv()
	server := httpapi.New(cfg, store.NewMemory(), runner.New())

	log.Printf("wreckr api listening on %s", cfg.Addr)
	if err := http.ListenAndServe(cfg.Addr, server.Handler()); err != nil {
		log.Fatal(err)
	}
}
