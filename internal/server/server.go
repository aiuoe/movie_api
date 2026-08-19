package server

import (
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"

	"github.com/local/movie_api/internal/config"
	"github.com/local/movie_api/internal/handlers"
	"github.com/local/movie_api/internal/store"
)

// New construye la app de Fiber con middleware y rutas registradas.
func New(cfg config.Config, st store.Store) *fiber.App {
	app := fiber.New(fiber.Config{
		AppName:               "movie_api",
		DisableStartupMessage: true,
	})

	app.Use(recover.New())
	app.Use(logger.New())
	app.Use(cors.New(cors.Config{
		AllowOrigins: "http://localhost:5173,http://localhost:4173",
		AllowHeaders: "Origin,Content-Type,Accept,Range",
		AllowMethods: "GET,POST,PUT,DELETE,OPTIONS",
	}))

	h := handlers.New(cfg, st)
	app.Get("/healthz", h.Health)
	app.Get("/api/media/trending", h.Trending)
	app.Get("/api/media/movies", h.Movies)
	app.Get("/api/media/series", h.Series)
	app.Get("/api/media/hero", h.Hero)
	app.Get("/api/media/top10", h.Top10)
	app.Get("/api/media/:id", h.ByID)
	app.Get("/api/media/:id/stream", h.Stream)
	app.Get("/api/me/continue", h.ContinueWatching)
	app.Get("/api/search", h.Search)
	app.Get("/api/lookup", h.Lookup)
	app.Get("/api/livetv", h.LiveTV)
	app.Post("/api/request", h.Request)
	app.Post("/api/jobs", h.EnqueueJob)

	return app
}