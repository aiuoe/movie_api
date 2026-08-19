package handlers

import (
	"github.com/gofiber/fiber/v2"
)

// LiveTV devuelve la lista de canales de Threadfin. Proxy simple al API
// interno de Threadfin — el SPA no necesita saber la URL del tuner.
func (h *Handlers) LiveTV(c *fiber.Ctx) error {
	// Threadfin expone /api/channels como JSON.
	url := "http://threadfin:34400/api/channels"

	status, body, errs := fiber.AcquireAgent().Get(url).Bytes()
	if len(errs) > 0 {
		return fiber.NewError(fiber.StatusBadGateway, errs[0].Error())
	}
	if status != 200 {
		// Si Threadfin no tiene canales configurados, devolvemos []
		// en vez de propagar el error — la UI lo maneja como "no hay canales".
		return c.JSON([]map[string]any{})
	}
	c.Set("Content-Type", "application/json")
	return c.Send(body)
}
</content>