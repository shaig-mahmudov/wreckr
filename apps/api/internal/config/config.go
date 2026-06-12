package config

import (
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Addr       string
	RunTimeout time.Duration

	MaxBodyBytes int64
	Guardrails   Guardrails

	StoreBackend string
	DatabaseURL  string

	RedisAddr         string
	WorkerConcurrency int
}

type Guardrails struct {
	MaxConcurrency      int
	MaxRequestRate      int
	MaxRunDuration      time.Duration
	MaxRequestBodyBytes int64
	TargetAllowlist     []string
}

func FromEnv() Config {
	runTimeout := time.Duration(envInt("WRECKR_RUN_TIMEOUT_SECONDS", 300)) * time.Second
	maxBodyBytes := int64(envInt("WRECKR_MAX_BODY_BYTES", 1<<20))
	return Config{
		Addr:         envString("WRECKR_API_ADDR", ":8080"),
		RunTimeout:   runTimeout,
		MaxBodyBytes: maxBodyBytes,
		Guardrails: Guardrails{
			MaxConcurrency:      envInt("WRECKR_MAX_CONCURRENCY", 1000),
			MaxRequestRate:      envInt("WRECKR_MAX_REQUEST_RATE_PER_SECOND", 5000),
			MaxRunDuration:      time.Duration(envInt("WRECKR_MAX_RUN_DURATION_SECONDS", int(runTimeout.Seconds()))) * time.Second,
			MaxRequestBodyBytes: int64(envInt("WRECKR_MAX_REQUEST_BODY_BYTES", int(maxBodyBytes))),
			TargetAllowlist:     envCSV("WRECKR_TARGET_ALLOWLIST"),
		},
		StoreBackend:      envString("WRECKR_STORE", "memory"),
		DatabaseURL:       envString("DATABASE_URL", "postgres://wreckr:wreckr@localhost:5432/wreckr?sslmode=disable"),
		RedisAddr:         envString("REDIS_ADDR", "localhost:6379"),
		WorkerConcurrency: envInt("WRECKR_WORKER_CONCURRENCY", 4),
	}
}

func envString(key string, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func envInt(key string, fallback int) int {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func envCSV(key string) []string {
	value := os.Getenv(key)
	if value == "" {
		return nil
	}
	parts := strings.Split(value, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}
