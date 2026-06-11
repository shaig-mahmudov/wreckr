package main

import (
	"context"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/wreckr/wreckr/apps/api/internal/config"
	"github.com/wreckr/wreckr/apps/api/internal/httpapi"
	"github.com/wreckr/wreckr/apps/api/internal/runner"
	"github.com/wreckr/wreckr/apps/api/internal/store"
)

func main() {
	cfg := config.FromEnv()
	st, err := openStore(cfg)
	if err != nil {
		log.Fatal(err)
	}
	server := httpapi.New(cfg, st, runner.New())

	log.Printf("wreckr api listening on %s", cfg.Addr)
	if err := http.ListenAndServe(cfg.Addr, server.Handler()); err != nil {
		log.Fatal(err)
	}
}

func openStore(cfg config.Config) (store.Store, error) {
	switch strings.ToLower(cfg.StoreBackend) {
	case "", "memory":
		return store.NewMemory(), nil
	case "postgres", "postgresql":
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return store.NewPostgres(ctx, cfg.DatabaseURL)
	default:
		return nil, &unknownStoreError{backend: cfg.StoreBackend}
	}
}

type unknownStoreError struct {
	backend string
}

func (e *unknownStoreError) Error() string {
	return "unknown WRECKR_STORE backend: " + e.backend
}
