package app

import (
	"context"
	"strings"
	"time"

	"github.com/wreckr/wreckr/apps/api/internal/blob"
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

func OpenBlobStore(ctx context.Context, cfg config.Config) (blob.Store, error) {
	if strings.ToLower(cfg.StoreBackend) == "memory" || cfg.S3.Endpoint == "" {
		return blob.NewMemoryStore(), nil
	}
	return blob.NewS3Store(ctx, cfg.S3)
}

type UnknownStoreError struct {
	Backend string
}

func (e *UnknownStoreError) Error() string {
	return "unknown WRECKR_STORE backend: " + e.Backend
}
