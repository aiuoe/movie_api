package handlers

import (
	"github.com/gofiber/fiber/v2"

	"github.com/local/movie_api/pkg/client"
)

// Stream proxy al storage final (filesystem local, NFS, S3, Jellyfin transcode).
// Por ahora devuelve 404 con mensaje claro — el SPA entiende esto y muestra
// “Sin fuente todavía”.
func (h *Handlers) Stream(c *fiber.Ctx) error {
	id := c.Params("id")
	if _, ok := h.st.ByID(id); !ok {
		return fiber.NewError(fiber.StatusNotFound, "media not found")
	}

	// Cuando enchufemos el storage real, este handler hará:
	//   1. Resolver URL del archivo fuente (Jellyfin / NFS / S3)
	//   2. Si el cliente pide Range, pasarlo al backend
	//   3. Si es video > 1080p, redirigir al transcode del worker
	//   4. Stream con Content-Type y soporte de seek
	//
	// Por ahora delegamos al worker para que sepa que hay un nuevo stream
	// por catalogar.
	_ = client.New(h.cfg.WorkerURL).NotifyStreamReady(id)
	return fiber.NewError(fiber.StatusNotFound, "stream not indexed yet — enqueue a transcode job")
}