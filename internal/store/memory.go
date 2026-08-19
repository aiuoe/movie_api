package store

import (
	"errors"
	"strings"
	"sync"

	"github.com/local/movie_api/internal/model"
)

// Memory es una implementación in-memory del Store. Datos sembrados con
// la misma lógica que `mockMedia.js` del SPA — así el front y el back
// comparten la misma fuente de verdad durante el desarrollo.
type Memory struct {
	mu     sync.RWMutex
	byID   map[string]model.Media
	hero   model.Hero
	cw     []model.ContinueWatching
}

func NewMemory() *Memory {
	return &Memory{
		byID: make(map[string]model.Media),
	}
}

func (m *Memory) Seed() error {
	seeds := seedMedia()
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, s := range seeds {
		m.byID[s.ID] = s
	}
	m.hero = model.Hero{
		Media:   seeds[0],
		Tagline: "Una historia que cruza el tiempo y la memoria.",
	}
	m.cw = []model.ContinueWatching{
		{Media: seeds[0], Progress: 0.42, Episode: "T1 · E3"},
		{Media: seeds[2], Progress: 0.78, Episode: "T2 · E5"},
		{Media: seeds[4], Progress: 0.15, Episode: "T1 · E1"},
		{Media: seeds[6], Progress: 0.93, Episode: "T3 · E8"},
	}
	return nil
}

func (m *Memory) Trending() []model.Media { return m.list("") }
func (m *Memory) Movies() []model.Media   { return m.list(string(model.KindMovie)) }
func (m *Memory) Series() []model.Media   { return m.list(string(model.KindSeries)) }

func (m *Memory) list(kind string) []model.Media {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]model.Media, 0, len(m.byID))
	for _, v := range m.byID {
		if kind == "" || string(v.Kind) == kind {
			out = append(out, v)
		}
	}
	return out
}

func (m *Memory) Hero() (model.Hero, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.hero, true
}

func (m *Memory) Top10() []model.Media {
	all := m.list("")
	if len(all) > 10 {
		return all[:10]
	}
	return all
}

func (m *Memory) ByID(id string) (model.Media, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	v, ok := m.byID[id]
	if !ok {
		return model.Media{}, false
	}
	return v, true
}

func (m *Memory) ContinueWatching() []model.ContinueWatching {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]model.ContinueWatching, len(m.cw))
	copy(out, m.cw)
	return out
}

func (m *Memory) Search(q string) []model.Media {
	q = strings.ToLower(strings.TrimSpace(q))
	if q == "" {
		return nil
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	var out []model.Media
	for _, v := range m.byID {
		if strings.Contains(strings.ToLower(v.Title), q) {
			out = append(out, v)
			continue
		}
		for _, g := range v.Genres {
			if strings.Contains(strings.ToLower(g), q) {
				out = append(out, v)
				break
			}
		}
	}
	return out
}

// seedMedia devuelve el mismo set que el mock del SPA. Cuando enchufemos el
// data source real (Jellyfin + Radarr/Sonarr), esta función se reemplaza por
// un cliente HTTP que lea de allí.
func seedMedia() []model.Media {
	img := func(seed string, w, h int) string {
		return "https://picsum.photos/seed/" + seed + "/" + itoa(w) + "/" + itoa(h)
	}
	backdrop := func(seed string) string {
		return "https://picsum.photos/seed/" + seed + "-bg/1920/1080"
	}
	mk := func(id, title string, year int, rating float64, dur, kind string, genres []string, seed string) model.Media {
		return model.Media{
			ID:       id,
			Title:    title,
			Year:     year,
			Rating:   rating,
			Duration: dur,
			Kind:     model.Kind(kind),
			Genres:   genres,
			Poster:   img("m-"+id, 800, 1200),
			Backdrop: backdrop(seed),
			Overview: "Lorem ipsum. Película / serie del catálogo local con metadata scrapeada automáticamente por el stack *arr.",
			Source:   "/api/media/" + id + "/stream",
		}
	}
	return []model.Media{
		mk("tt-001", "Neón Silencioso", 2024, 8.4, "2h 14m", "movie", []string{"Sci-Fi", "Thriller"}, "neon"),
		mk("tt-002", "La Última Estación", 2023, 7.9, "1h 58m", "movie", []string{"Drama"}, "station"),
		mk("tt-003", "Código Eclipse", 2024, 8.1, "Serie", "series", []string{"Sci-Fi", "Misterio"}, "eclipse"),
		mk("tt-004", "Hielo Rojo", 2022, 7.4, "1h 47m", "movie", []string{"Acción"}, "ice"),
		mk("tt-005", "Senderos", 2024, 8.0, "Serie", "series", []string{"Drama", "Aventura"}, "trail"),
		mk("tt-006", "Niebla", 2023, 7.2, "1h 33m", "movie", []string{"Terror"}, "mist"),
		mk("tt-007", "Voltaje", 2025, 8.6, "Serie", "series", []string{"Acción", "Sci-Fi"}, "volt"),
		mk("tt-008", "Crisálida", 2024, 7.8, "2h 02m", "movie", []string{"Drama", "Romance"}, "cris"),
		mk("tt-009", "Mar de Estrellas", 2023, 8.3, "Serie", "series", []string{"Aventura", "Sci-Fi"}, "sea"),
		mk("tt-010", "Raíz", 2024, 7.6, "1h 50m", "movie", []string{"Thriller"}, "root"),
	}
}

// itoa rápido sin strconv para mantener el seed legible.
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

var _ = errors.New