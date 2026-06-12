package app

import (
	"context"
	"strings"
	"time"

	"github.com/wreckr/wreckr/apps/api/internal/config"
	"github.com/wreckr/wreckr/apps/api/internal/store"
)

func OpenStore(cfg config.Config) (store.Store, error) {
	switch strings.ToLower(cfg.StoreBackend) {
	case "", "memory":
		return store.NewMemory(), nil
	case "postgres", "postgresql":
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return store.NewPostgres(ctx, cfg.DatabaseURL)
	default:
		return nil, &UnknownStoreError{Backend: cfg.StoreBackend}
	}
}

type UnknownStoreError struct {
	Backend string
}

func (e *UnknownStoreError) Error() string {
	return "unknown WRECKR_STORE backend: " + e.Backend
}
