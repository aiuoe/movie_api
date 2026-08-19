package config

import "os"

// Config agrupa los parámetros runtime del API.
// Todos los valores tienen defaults sensatos; se sobreescriben por env.
type Config struct {
	Addr        string
	WorkerURL   string
	JWTSecret   string
	Bucket      string
	StorageRoot string
	RadarrURL   string
	RadarrKey   string
	SonarrURL   string
	SonarrKey   string
}

func Load() Config {
	return Config{
		Addr:        env("ADDR", ":8080"),
		WorkerURL:   env("WORKER_URL", "http://localhost:9090"),
		JWTSecret:   env("JWT_SECRET", "dev-secret-change-me"),
		Bucket:      env("S3_BUCKET", "media"),
		StorageRoot: env("STORAGE_ROOT", ""),
		RadarrURL:   env("RADARR_URL", "http://localhost:7878"),
		RadarrKey:   env("RADARR_API_KEY", ""),
		SonarrURL:   env("SONARR_URL", "http://localhost:8989"),
		SonarrKey:   env("SONARR_API_KEY", ""),
	}
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}