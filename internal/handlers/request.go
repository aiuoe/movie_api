package handlers

import (
	"strings"

	"github.com/gofiber/fiber/v2"

	"github.com/local/movie_api/internal/arr"
)

// RequestBody es lo que manda la SPA cuando el usuario hace "Pedir".
type RequestBody struct {
	Kind     string `json:"kind"`     // "movie" | "series"
	Title    string `json:"title"`
	Year     int    `json:"year"`
	ExternalID uint  `json:"external_id"` // TMDB id (movie) o TVDB id (series)
}

// LookupResponse es lo que devuelve /api/lookup — el cliente lo usa
// para mostrar la card antes de pedir.
type LookupResponse struct {
	Results []map[string]any `json:"results"`
}

// Lookup: busca títulos en TMDB a través de Radarr/Sonarr.
// /api/lookup?kind=movie&q=dune&year=2024
func (h *Handlers) Lookup(c *fiber.Ctx) error {
	kind := c.Query("kind", "movie")
	q := strings.TrimSpace(c.Query("q"))
	if q == "" {
		return c.JSON(LookupResponse{Results: []map[string]any{}})
	}

	var path string
	switch kind {
	case "series":
		path = "/api/v3/series/lookup?term=" + arr.Q(q)
	case "movie":
		fallthrough
	default:
		path = "/api/v3/movie/lookup?term=" + arr.Q(q)
	}

	var raw []map[string]any
	if err := h.arrFor(kind).Get(path, &raw); err != nil {
		// Si el arr no está configurado aún, devolvemos un set vacío
		// para que la UI no rompa. El usuario verá "No se pudo buscar — falta API key".
		return c.JSON(LookupResponse{Results: []map[string]any{}})
	}

	// Achicamos la respuesta a lo que la SPA necesita.
	out := make([]map[string]any, 0, len(raw))
	for _, m := range raw {
		title, _ := m["title"].(string)
		year := yearFrom(m["year"])
		overview, _ := m["overview"].(string)
		poster := posterFrom(m["remotePoster"])
		external := externalFrom(kind, m)

		out = append(out, map[string]any{
			"title":       title,
			"year":        year,
			"overview":    overview,
			"poster":      poster,
			"external_id": external,
			"kind":        kind,
		})
	}
	return c.JSON(LookupResponse{Results: out})
}

// Request: el usuario pide un título — el API lo manda a Radarr/Sonarr.
func (h *Handlers) Request(c *fiber.Ctx) error {
	var req RequestBody
	if err := c.BodyParser(&req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid body")
	}
	if req.Title == "" || req.ExternalID == 0 {
		return fiber.NewError(fiber.StatusBadRequest, "title and external_id required")
	}

	var arrPath string
	var body map[string]any
	switch req.Kind {
	case "series":
		arrPath = "/api/v3/series"
		body = map[string]any{
			"title":        req.Title,
			"qualityProfileId": 1,
			"tvdbId":       req.ExternalID,
			"rootFolderPath": "/downloads/tv",
			"monitored":    true,
			"addOptions":   map[string]any{"searchForMissingEpisodes": true},
		}
	case "movie":
		fallthrough
	default:
		arrPath = "/api/v3/movie"
		body = map[string]any{
			"title":        req.Title,
			"qualityProfileId": 1,
			"tmdbId":       req.ExternalID,
			"rootFolderPath": "/downloads/movies",
			"monitored":    true,
			"minimumAvailability": "released",
			"addOptions":   map[string]any{"searchForMovie": true},
		}
	}

	if err := h.arrFor(req.Kind).Post(arrPath, body, nil); err != nil {
		return fiber.NewError(fiber.StatusBadGateway, err.Error())
	}

	return c.Status(fiber.StatusAccepted).JSON(fiber.Map{
		"status":  "requested",
		"title":   req.Title,
		"message": "Title sent to " + req.Kind + " — it'll appear when download completes",
	})
}

func (h *Handlers) arrFor(kind string) *arr.Client {
	if kind == "series" {
		return arr.New(h.cfg.SonarrURL, h.cfg.SonarrKey)
	}
	return arr.New(h.cfg.RadarrURL, h.cfg.RadarrKey)
}

func yearFrom(v any) int {
	switch n := v.(type) {
	case float64:
		return int(n)
	case int:
		return n
	}
	return 0
}

func posterFrom(v any) string {
	m, ok := v.(map[string]any)
	if !ok {
		return ""
	}
	s, _ := m["url"].(string)
	return s
}

func externalFrom(kind string, m map[string]any) uint {
	if kind == "series" {
		if v, ok := m["tvdbId"].(float64); ok {
			return uint(v)
		}
	} else {
		if v, ok := m["tmdbId"].(float64); ok {
			return uint(v)
		}
	}
	return 0
}