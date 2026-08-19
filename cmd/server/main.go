package main

import (
	"log"

	"github.com/local/movie_api/internal/config"
	"github.com/local/movie_api/internal/server"
	"github.com/local/movie_api/internal/store"
)

func main() {
	cfg := config.Load()
	st := store.NewMemory()
	if err := st.Seed(); err != nil {
		log.Fatalf("seed store: %v", err)
	}

	app := server.New(cfg, st)

	log.Printf("movie_api listening on %s (worker=%s)", cfg.Addr, cfg.WorkerURL)
	if err := app.Listen(cfg.Addr); err != nil {
		log.Fatalf("listen: %v", err)
	}
}