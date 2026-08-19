package config

import (
	"os"
)

// Config agrupa los parámetros runtime del API.
// Todos los valores tienen defaults sensatos; se sobreescriben por env.
type Config struct {
	Addr      string // puerto del API
	WorkerURL string // URL base de movie_worker
	JWTSecret string // placeholder para cuando agreguemos auth
}

func Load() Config {
	return Config{
		Addr:      env("ADDR", ":8080"),
		WorkerURL: env("WORKER_URL", "http://localhost:9090"),
		JWTSecret: env("JWT_SECRET", "dev-secret-change-me"),
	}
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}