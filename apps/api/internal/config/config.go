package config

import (
	"os"
	"strconv"
	"time"
)

type Config struct {
	Addr         string
	RunTimeout   time.Duration
	MaxBodyBytes int64
}

func FromEnv() Config {
	return Config{
		Addr:         envString("WRECKR_API_ADDR", ":8080"),
		RunTimeout:   time.Duration(envInt("WRECKR_RUN_TIMEOUT_SECONDS", 300)) * time.Second,
		MaxBodyBytes: int64(envInt("WRECKR_MAX_BODY_BYTES", 1<<20)),
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
