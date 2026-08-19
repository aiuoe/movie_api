package handlers

import (
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"

	"github.com/local/movie_api/internal/model"
	"github.com/local/movie_api/pkg/client"
)

// EnqueueJob recibe la petición del SPA (o admin) y la reenvía al worker.
// En producción añadiríamos persistencia local para reintentos.
func (h *Handlers) EnqueueJob(c *fiber.Ctx) error {
	var req model.JobRequest
	if err := c.BodyParser(&req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid body")
	}
	if req.Kind == "" || req.MediaID == "" {
		return fiber.NewError(fiber.StatusBadRequest, "kind and media_id required")
	}

	wc := client.New(h.cfg.WorkerURL)
	jobID, err := wc.Submit(req)
	if err != nil {
		// No bloqueamos al cliente — encolamos un reintento local con el mismo uuid.
		jobID = uuid.NewString()
		go func(req model.JobRequest, jobID string) {
			time.Sleep(2 * time.Second)
			_, _ = wc.Submit(req)
		}(req, jobID)
		return c.Status(fiber.StatusAccepted).JSON(model.JobResponse{
			JobID:  jobID,
			Status: "queued-retry",
		})
	}

	return c.Status(fiber.StatusAccepted).JSON(model.JobResponse{
		JobID:  jobID,
		Status: "queued",
	})
}