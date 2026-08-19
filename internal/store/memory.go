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
	// El primer item con StorageKey es el "real" (Big Buck Bunny).
	var demo model.Media
	for _, s := range seeds {
		if s.StorageKey != "" {
			demo = s
			break
		}
	}
	if demo.ID == "" {
		demo = seeds[0]
	}
	m.hero = model.Hero{
		Media:   demo,
		Tagline: "Demo del pipeline · Big Buck Bunny (CC-BY) en MinIO",
	}
	m.cw = []model.ContinueWatching{
		{Media: demo, Progress: 0.0, Episode: "T1 · E1"},
		{Media: seeds[0], Progress: 0.42, Episode: "T1 · E3"},
		{Media: seeds[2], Progress: 0.78, Episode: "T2 · E5"},
		{Media: seeds[4], Progress: 0.15, Episode: "T1 · E1"},
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
	return v, ok
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

// seedMedia devuelve un set amplio de muestra. Sólo un item tiene StorageKey
// real (el demo Big Buck Bunny en el bucket). El resto son placeholders que
// aparecerán como "no disponible" en el player hasta que Radarr/Sonarr bajen
// el archivo real y el webhook lo suba a MinIO.
func seedMedia() []model.Media {
	img := func(seed string, w, h int) string {
		return "https://picsum.photos/seed/" + seed + "/" + itoa(w) + "/" + itoa(h)
	}
	backdrop := func(seed string) string {
		return "https://picsum.photos/seed/" + seed + "-bg/1920/1080"
	}
	mk := func(id, title string, year int, rating float64, dur, kind, lang string, genres []string, seed string, withStorage bool) model.Media {
		m := model.Media{
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
			Lang:     lang,
		}
		if withStorage {
			m.StorageKey = id + "/" + lang + "/source.mp4"
		}
		return m
	}
	return []model.Media{
		// ── DEMO REAL (storage_key) ───────────────────────────────────────
		mk("got-s01e01", "Big Buck Bunny — demo del pipeline", 2008, 8.0, "10m", "movie", "es-ES", []string{"Animación", "Comedia"}, "demo", true),

		// ── Series (placeholder) ─────────────────────────────────────────
		mk("got-001", "Juego de Tronos", 2011, 9.2, "Serie", "series", "es-ES", []string{"Fantasía", "Drama"}, "got-001", false),
		mk("brk-001", "Breaking Bad", 2008, 9.5, "Serie", "series", "es-ES", []string{"Drama", "Thriller"}, "brk-001", false),
		mk("str-001", "Stranger Things", 2016, 8.7, "Serie", "series", "es-ES", []string{"Sci-Fi", "Terror"}, "str-001", false),
		mk("mnd-001", "The Mandalorian", 2019, 8.7, "Serie", "series", "es-ES", []string{"Sci-Fi", "Aventura"}, "mnd-001", false),
		mk("wed-001", "The Witcher", 2019, 8.2, "Serie", "series", "es-ES", []string{"Fantasía"}, "wed-001", false),
		mk("dpr-001", "Dark", 2017, 8.8, "Serie", "series", "es-ES", []string{"Sci-Fi", "Misterio"}, "dpr-001", false),
		mk("hcd-001", "House of the Dragon", 2022, 8.4, "Serie", "series", "es-ES", []string{"Fantasía"}, "hcd-001", false),
		mk("thl-001", "The Last of Us", 2023, 8.7, "Serie", "series", "es-ES", []string{"Drama", "Sci-Fi"}, "thl-001", false),
		mk("wht-001", "Severance", 2022, 8.7, "Serie", "series", "es-ES", []string{"Sci-Fi", "Thriller"}, "wht-001", false),

		// ── Películas (placeholder) ───────────────────────────────────────
		mk("inc-001", "Inception", 2010, 8.8, "2h 28m", "movie", "es-ES", []string{"Sci-Fi", "Thriller"}, "inc-001", false),
		mk("int-001", "Interstellar", 2014, 8.7, "2h 49m", "movie", "es-ES", []string{"Sci-Fi", "Drama"}, "int-001", false),
		mk("dun-001", "Dune", 2021, 8.0, "2h 35m", "movie", "es-ES", []string{"Sci-Fi", "Aventura"}, "dun-001", false),
		mk("opn-001", "Oppenheimer", 2023, 8.4, "3h 00m", "movie", "es-ES", []string{"Drama", "Historia"}, "opn-001", false),
		mk("bar-001", "Barbie", 2023, 6.9, "1h 54m", "movie", "es-ES", []string{"Comedia"}, "bar-001", false),
		mk("top-001", "Top Gun: Maverick", 2022, 8.3, "2h 11m", "movie", "es-ES", []string{"Acción"}, "top-001", false),
		mk("jok-001", "Joker", 2019, 8.4, "2h 02m", "movie", "es-ES", []string{"Drama", "Thriller"}, "jok-001", false),
		mk("par-001", "Parasite", 2019, 8.5, "2h 12m", "movie", "es-ES", []string{"Thriller", "Drama"}, "par-001", false),
		mk("avt-001", "Avengers: Endgame", 2019, 8.4, "3h 01m", "movie", "es-ES", []string{"Acción", "Sci-Fi"}, "avt-001", false),
		mk("shn-001", "The Shawshank Redemption", 1994, 9.3, "2h 22m", "movie", "es-ES", []string{"Drama"}, "shn-001", false),
		mk("gmf-001", "The Godfather", 1972, 9.2, "2h 55m", "movie", "es-ES", []string{"Drama", "Crimen"}, "gmf-001", false),
		mk("dpk-001", "Pulp Fiction", 1994, 8.9, "2h 34m", "movie", "es-ES", []string{"Drama", "Crimen"}, "dpk-001", false),
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