package config

import (
	"os"
)

type Config struct {
	HTTPAddr    string
	DatabaseURL string
	JWTSecret   string
	SeedOnStart bool
}

func Load() Config {
	c := Config{
		HTTPAddr:    env("HTTP_ADDR", ":8080"),
		DatabaseURL: env("DATABASE_URL", "postgres://pvapp:pvapp@localhost:5433/pvplatform?sslmode=disable"),
		JWTSecret:   env("JWT_SECRET", "dev-secret-change-me"),
		SeedOnStart: env("SEED_ON_START", "true") == "true",
	}
	return c
}

func env(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}
