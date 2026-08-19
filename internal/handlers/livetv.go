package handlers

import (
	"io"
	"net/http"
	"time"

	"github.com/gofiber/fiber/v2"
)

// LiveTV devuelve la lista de canales de Threadfin. Proxy simple al API
// interno de Threadfin — el SPA no necesita saber la URL del tuner.
func (h *Handlers) LiveTV(c *fiber.Ctx) error {
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get("http://threadfin:34400/api/channels")
	if err != nil {
		// Si Threadfin está caído o no tiene canales, devolvemos []
		// en vez de propagar el error — la UI lo maneja como "sin canales".
		return c.JSON([]map[string]any{})
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return c.JSON([]map[string]any{})
	}
	body, _ := io.ReadAll(resp.Body)
	c.Set("Content-Type", "application/json")
	return c.Send(body)
}