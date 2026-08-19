package handlers

import (
	"strings"

	"github.com/gofiber/fiber/v2"

	"github.com/local/movie_api/internal/model"
)

func (h *Handlers) Trending(c *fiber.Ctx) error {
	return c.JSON(h.st.Trending())
}

func (h *Handlers) Movies(c *fiber.Ctx) error {
	return c.JSON(h.st.Movies())
}

func (h *Handlers) Series(c *fiber.Ctx) error {
	return c.JSON(h.st.Series())
}

func (h *Handlers) Hero(c *fiber.Ctx) error {
	hero, ok := h.st.Hero()
	if !ok {
		return fiber.NewError(fiber.StatusNotFound, "no hero configured")
	}
	return c.JSON(hero)
}

func (h *Handlers) Top10(c *fiber.Ctx) error {
	return c.JSON(h.st.Top10())
}

func (h *Handlers) ByID(c *fiber.Ctx) error {
	id := c.Params("id")
	m, ok := h.st.ByID(id)
	if !ok {
		return fiber.NewError(fiber.StatusNotFound, "media not found")
	}
	return c.JSON(enrich(m))
}

// enrich añade campos derivados para que el SPA no tenga que pedirlos aparte.
// En la versión “Jellyfin” esto se reemplaza por llamadas a /Items/{id}…
func enrich(m model.Media) model.Media {
	if m.Cast != nil || m.Director != "" {
		return m
	}
	m.Cast = []string{"A. Vega", "L. Cruz", "M. Soto", "R. Lima", "C. Reyes"}
	m.Director = "I. Martín"
	if m.Kind == model.KindSeries {
		m.Seasons = 3
		eps := make([]model.Episode, 8)
		for i := range eps {
			eps[i] = model.Episode{
				ID:       m.ID + "-e" + itoa(i+1),
				Title:    "Episodio " + itoa(i+1),
				Duration: "45m",
				Thumb:    "https://picsum.photos/seed/ep-" + m.ID + "-" + itoa(i) + "/400/220",
				Overview: "Lorem ipsum del episodio. Sinopsis scrapeada por Sonarr.",
			}
		}
		m.Episodes = map[string][]model.Episode{"1": eps}
	}
	return m
}

func (h *Handlers) ContinueWatching(c *fiber.Ctx) error {
	return c.JSON(h.st.ContinueWatching())
}

func (h *Handlers) Search(c *fiber.Ctx) error {
	q := strings.TrimSpace(c.Query("q"))
	if q == "" {
		return c.JSON([]model.Media{})
	}
	return c.JSON(h.st.Search(q))
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}