package store

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/local/movie_api/internal/model"
)

// Memory es una implementación in-memory del Store.
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

	// Hero = el estreno más reciente que esté disponible.
	// Si no hay estreno disponible, usar el primer item con StorageKey.
	var heroItem model.Media
	var heroYear int
	for _, s := range seeds {
		disponible := s.StorageKey != ""
		esEstreno := s.IsEstreno
		if disponible && esEstreno && s.Year > heroYear {
			heroItem = s
			heroYear = s.Year
		}
	}
	if heroItem.ID == "" {
		for _, s := range seeds {
			if s.StorageKey != "" {
				heroItem = s
				break
			}
		}
	}
	if heroItem.ID == "" {
		heroItem = seeds[0]
	}
	m.hero = model.Hero{Media: heroItem}

	m.cw = []model.ContinueWatching{
		{Media: seeds[0], Progress: 0.42, Episode: "T1 · E3"},
		{Media: seeds[2], Progress: 0.78, Episode: "T2 · E5"},
		{Media: seeds[4], Progress: 0.15, Episode: "T1 · E1"},
	}
	return nil
}

// Trending devuelve todo, ordenado: primero disponibles (recientes), luego no-disponibles.
func (m *Memory) Trending() []model.Media {
	all := m.list("")
	sort.Slice(all, func(i, j int) bool {
		di, dj := all[i].StorageKey != "", all[j].StorageKey != ""
		if di != dj {
			return di
		}
		return all[i].Year > all[j].Year
	})
	return all
}

func (m *Memory) Movies() []model.Media {
	out := m.list(string(model.KindMovie))
	sort.Slice(out, func(i, j int) bool {
		return out[i].Year > out[j].Year
	})
	return out
}

func (m *Memory) Series() []model.Media {
	out := m.list(string(model.KindSeries))
	sort.Slice(out, func(i, j int) bool {
		return out[i].Year > out[j].Year
	})
	return out
}

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
	all := m.Trending()
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
	sort.Slice(out, func(i, j int) bool {
		return out[i].Year > out[j].Year
	})
	return out
}

// tmdbURL devuelve la URL del poster en TMDB. Si el path no existe, retorna
// un placeholder de placehold.co con el título codificado.
func tmdbURL(path, title string, w int) string {
	if path != "" {
		return "https://image.tmdb.org/t/p/w" + itoa(w) + path
	}
	return fmt.Sprintf("https://placehold.co/%dx%d/262626/eee?text=%s", w, w*3/2, urlQuery(title))
}

func backdropURL(path, title string, w int) string {
	if path != "" {
		return "https://image.tmdb.org/t/p/w" + itoa(w) + path
	}
	return fmt.Sprintf("https://placehold.co/%dx%d/171717/999?text=%s", w, w*9/16, urlQuery(title))
}

func urlQuery(s string) string {
	var out strings.Builder
	for _, r := range s {
		switch {
		case r == ' ':
			out.WriteString("+")
		case r >= 'A' && r <= 'Z', r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			out.WriteRune(r)
		default:
			fmt.Fprintf(&out, "%%%02X", r)
		}
	}
	return out.String()
}

// seedMedia: usa posters reales de TMDB para los verificados + placehold.co para el resto.
// is_estreno = año >= 2024.
func seedMedia() []model.Media {
	// Películas con posters VERIFICADOS
	p := func(id, title string, year int, rating float64, dur, lang, poster, backdrop string, genres []string, withStorage bool) model.Media {
		return model.Media{
			ID:       id,
			Title:    title,
			Year:     year,
			Rating:   rating,
			Duration: dur,
			Kind:     model.Kind("movie"),
			Genres:   genres,
			Poster:   tmdbURL(poster, title, 500),
			Backdrop: backdropURL(backdrop, title, 1280),
			Overview: "Película del catálogo local. Metadata scrapeada automáticamente por el stack *arr.",
			Lang:     lang,
			IsEstreno: year >= 2024,
		}
	}
	s := func(id, title string, year int, rating float64, lang, poster, backdrop string, genres []string, withStorage bool) model.Media {
		return model.Media{
			ID:       id,
			Title:    title,
			Year:     year,
			Rating:   rating,
			Duration: "Serie",
			Kind:     model.Kind("series"),
			Genres:   genres,
			Poster:   tmdbURL(poster, title, 500),
			Backdrop: backdropURL(backdrop, title, 1280),
			Overview: "Serie del catálogo local. Metadata scrapeada automáticamente por el stack *arr.",
			Lang:     lang,
			IsEstreno: year >= 2024,
		}
	}

	return []model.Media{
		// ── ÚNICO demo REAL (storage_key → bucket) ───────────────────────
		{
			ID:        "demo-001",
			Title:     "Big Buck Bunny — demo del pipeline",
			Year:      2008,
			Rating:    8.0,
			Duration:  "10m",
			Kind:      model.Kind("movie"),
			Genres:    []string{"Animación", "Comedia"},
			Poster:    "https://placehold.co/500x750/262626/eee?text=Big+Buck+Bunny",
			Backdrop:  "https://placehold.co/1280x720/171717/999?text=Big+Buck+Bunny",
			Overview:  "Demo del pipeline end-to-end. Disponible en MinIO.",
			Lang:      "es-ES",
			IsEstreno: false,
			StorageKey: "demo-001/es-ES/source.mp4",
		},

		// ── Películas (ordenadas por año DESC — los más nuevos primero) ───
		p("mov-oppenheimer", "Oppenheimer",     2023, 8.4, "3h 00m", "es-ES",
		  "/8Gxv8gSFCU0XGDykEGv7zR1n2ua.jpg", "/fm6KqXpk3M2HVveHwCrBSSBaO0V.jpg",
		  []string{"Drama", "Historia"}, false),
		p("mov-barbie",      "Barbie",          2023, 6.9, "1h 54m", "es-ES",
		  "", "",
		  []string{"Comedia"}, false),
		p("mov-topgun",      "Top Gun: Maverick", 2022, 8.3, "2h 11m", "es-ES",
		  "/62HCnUTziyWcpDaBO2i1DX17ljH.jpg", "",
		  []string{"Acción"}, false),
		p("mov-dune",        "Dune",            2021, 8.0, "2h 35m", "es-ES",
		  "/d5NXSklXo0qyIYkgV94XAgMIckC.jpg", "/iopYFB1b6Bh7FWZh3onQhph1sih.jpg",
		  []string{"Sci-Fi", "Aventura"}, false),
		p("mov-parasite",    "Parasite",        2019, 8.5, "2h 12m", "es-ES",
		  "/7IiTTgloJzvGI1TAYymCfbfl3vT.jpg", "",
		  []string{"Thriller", "Drama"}, false),
		p("mov-avengers",    "Avengers: Endgame", 2019, 8.4, "3h 01m", "es-ES",
		  "/or06FN3Dka5tukK1e9sl16pB3iy.jpg", "",
		  []string{"Acción", "Sci-Fi"}, false),
		p("mov-joker",       "Joker",           2019, 8.4, "2h 02m", "es-ES",
		  "/udDclJoHjfjb8Ekgsd4FDteOkCU.jpg", "",
		  []string{"Drama", "Thriller"}, false),
		p("mov-interstellar","Interstellar",    2014, 8.7, "2h 49m", "es-ES",
		  "/gEU2QniE6E77NI6lCU6MxlNBvIx.jpg", "/pbrkL804c8yAv3zBZR4QPEafpAR.jpg",
		  []string{"Sci-Fi", "Drama"}, false),
		p("mov-inception",   "Inception",       2010, 8.8, "2h 28m", "es-ES",
		  "/9gk7adHYeDvHkCSEqAvQNLV5Uge.jpg", "/s3TBrRGB1iav7gFOCNx3H31MoES.jpg",
		  []string{"Sci-Fi", "Thriller"}, false),
		p("mov-pulpfiction", "Pulp Fiction",    1994, 8.9, "2h 34m", "es-ES",
		  "/d5iIlFn5s0ImszYzBPb8JPIfbXD.jpg", "",
		  []string{"Drama", "Crimen"}, false),
		p("mov-shawshank",   "Shawshank Redemption", 1994, 9.3, "2h 22m", "es-ES",
		  "/q6y0Go1tsGEsmtFryDOJo3dEmqu.jpg", "",
		  []string{"Drama"}, false),
		p("mov-fightclub",   "Fight Club",      1999, 8.8, "2h 19m", "es-ES",
		  "/pB8BM7pdSp6B6Ih7QZ4DrQ3PmJK.jpg", "",
		  []string{"Drama", "Thriller"}, false),
		p("mov-godfather",   "The Godfather",   1972, 9.2, "2h 55m", "es-ES",
		  "/3bhkrj58Vtu7enYsRolD1fZdja1.jpg", "",
		  []string{"Drama", "Crimen"}, false),

		// ── Series (ordenadas por año DESC) ──────────────────────────────
		s("ser-thelastofus", "The Last of Us",        2023, 8.7, "es-ES",
		  "/uKvVjHNqB5VmOrdxqAt2F7J78ED.jpg", "",
		  []string{"Drama", "Sci-Fi"}, false),
		s("ser-severance",   "Severance",            2022, 8.7, "es-ES",
		  "", "",
		  []string{"Sci-Fi", "Thriller"}, false),
		s("ser-hotd",        "House of the Dragon",   2022, 8.4, "es-ES",
		  "/7QMsOTMUswlwxJP0rTTZfmz2tX2.jpg", "",
		  []string{"Fantasía"}, false),
		s("ser-mandalorian", "The Mandalorian",       2019, 8.7, "es-ES",
		  "", "",
		  []string{"Sci-Fi", "Aventura"}, false),
		s("ser-witcher",     "The Witcher",           2019, 8.2, "es-ES",
		  "", "",
		  []string{"Fantasía"}, false),
		s("ser-stranger",    "Stranger Things",      2016, 8.7, "es-ES",
		  "/49WJfeN0moxb9IPfGn8AIqMGskD.jpg", "",
		  []string{"Sci-Fi", "Terror"}, false),
		s("ser-dark",        "Dark",                  2017, 8.8, "es-ES",
		  "", "",
		  []string{"Sci-Fi", "Misterio"}, false),
		s("ser-got",         "Game of Thrones",       2011, 9.2, "es-ES",
		  "", "",
		  []string{"Fantasía", "Drama"}, false),
		s("ser-breakingbad", "Breaking Bad",          2008, 9.5, "es-ES",
		  "", "",
		  []string{"Drama", "Thriller"}, false),
	}
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

var _ = errors.New
var _ = fmt.Sprintf