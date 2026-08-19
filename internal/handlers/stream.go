package handlers

import (
	"time"

	"github.com/gofiber/fiber/v2"

	"github.com/local/movie_api/pkg/client"
)

// Stream resuelve el `Source`/`StorageKey` de un media y devuelve una URL
// presigned al bucket (vía movie_worker) válida por 4h. El SPA la usa
// directo en el tag <video>, sin proxy.
func (h *Handlers) Stream(c *fiber.Ctx) error {
	id := c.Params("id")
	m, ok := h.st.ByID(id)
	if !ok {
		return fiber.NewError(fiber.StatusNotFound, "media not found")
	}

	key := m.StorageKey
	if key == "" {
		// Fallback: derivamos del id y el lang del media.
		lang := m.Lang
		if lang == "" {
			lang = "es-ES"
		}
		key = h.cfg.StorageRoot + "/" + id + "/" + lang + "/source.mp4"
	}

	wc := client.New(h.cfg.WorkerURL)
	url, err := wc.Presign(key, 4*time.Hour)
	if err != nil {
		// Si el worker está caído o el objeto no existe, devolvemos 404 con
		// mensaje claro — el SPA lo entiende como "no hay stream disponible".
		return fiber.NewError(fiber.StatusNotFound, "stream not ready: "+err.Error())
	}

	return c.JSON(fiber.Map{
		"url":       url,
		"expires":   time.Now().Add(4 * time.Hour).Unix(),
		"media_id":  id,
		"bucket":    h.cfg.Bucket,
		"key":       key,
	})
}