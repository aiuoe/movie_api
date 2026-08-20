package store

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

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

// posterSVG genera un poster inline (data URI) con el título y branding del
// provider. Sin requests externos — funciona en cualquier red.
func posterSVG(title string, providers []string) string {
	primary := "Streaming"
	bg := "#262626"
	fg := "#eeeeee"
	brand := "STREAMING"
	if len(providers) > 0 {
		primary = providers[0]
	}
	switch primary {
	case "Netflix":
		bg, fg, brand = "#e50914", "#ffffff", "NETFLIX"
	case "Disney+":
		bg, fg, brand = "#0f1e3d", "#ffffff", "DISNEY+"
	case "HBO Max":
		bg, fg, brand = "#9b51e0", "#ffffff", "HBO MAX"
	case "Paramount+":
		bg, fg, brand = "#0064ff", "#ffffff", "PARAMOUNT+"
	case "Hulu":
		bg, fg, brand = "#1ce783", "#000000", "HULU"
	case "Apple TV+":
		bg, fg, brand = "#000000", "#ffffff", "APPLE TV+"
	case "Prime Video":
		bg, fg, brand = "#1399ff", "#ffffff", "PRIME VIDEO"
	case "Demo":
		bg, fg, brand = "#404040", "#ffffff", "DEMO"
	}
	t := title
	if len([]rune(t)) > 40 {
		t = string([]rune(t)[:36]) + "…"
	}
	svg := fmt.Sprintf(`<svg xmlns='http://www.w3.org/2000/svg' viewBox='0 0 500 750'>
<rect width='500' height='750' fill='%s'/>
<rect x='20' y='20' width='460' height='710' fill='none' stroke='%s' stroke-opacity='0.3' stroke-width='1.5'/>
<text x='250' y='100' font-family='Inter,system-ui,sans-serif' font-size='28' font-weight='800' fill='%s' text-anchor='middle' letter-spacing='3'>%s</text>
<text x='250' y='375' font-family='Inter,system-ui,sans-serif' font-size='34' font-weight='700' fill='%s' text-anchor='middle'>%s</text>
<text x='250' y='720' font-family='Inter,system-ui,sans-serif' font-size='14' fill='%s' fill-opacity='0.6' text-anchor='middle'>movie_spa · self-hosted</text>
</svg>`,
		bg, fg, fg, brand, fg, escapeXML(t), fg)
	// data URI: url-encode # → %23, otros no importa en SVGBase
	return "data:image/svg+xml;utf8," + url.PathEscape(svg)
}

// escapeXML escapa < > & " ' para SVG inline
func escapeXML(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	return s
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
	items := []model.Media{
		// ── DEMO REAL (storage_key → bucket) ─────────────────────────────
		{
			ID:         "demo-001",
			Title:      "Big Buck Bunny — demo del pipeline",
			Year:       2008,
			Rating:     8.0,
			Duration:   "10m",
			Kind:       model.Kind("movie"),
			Genres:     []string{"Animación", "Comedia"},
			Poster:     "https://placehold.co/500x750/262626/eee?text=Big+Buck+Bunny",
			Backdrop:   "https://placehold.co/1280x720/171717/999?text=Big+Buck+Bunny",
			Overview:   "Demo del pipeline end-to-end. Disponible en MinIO.",
			Lang:       "es-ES",
			IsEstreno:  false,
			Providers:  []string{"Demo"},
			ExternalID: 0,
			StorageKey: "demo-001/es-ES/source.mp4",
		},

		// ── APPLE TV+ ──
		{
			ID:         "ap-sev-2",
			Title:      "Severance: T2",
			Year:       2025,
			Rating:     9.0,
			Duration:   "Serie",
			Kind:       model.Kind("series"),
			Genres:     []string{"Sci-Fi", "Drama"},
			Poster:     "https://placehold.co/500x750/000000/ffffff?text=APPLE+TV%2B%0A%0ASeverance%3A+T2",
			Backdrop:   "https://placehold.co/500x750/000000/ffffff?text=APPLE+TV%2B%0A%0ASeverance%3A+T2",
			Overview:   "Serie disponible en streaming.",
			Lang:       "es-ES",
			IsEstreno:  true,
			Providers:  []string{"Apple TV+"},
			ExternalID: 114472,
		},
		{
			ID:         "ap-silo-2",
			Title:      "Silo: T2",
			Year:       2024,
			Rating:     8.2,
			Duration:   "Serie",
			Kind:       model.Kind("series"),
			Genres:     []string{"Sci-Fi"},
			Poster:     "https://placehold.co/500x750/000000/ffffff?text=APPLE+TV%2B%0A%0ASilo%3A+T2",
			Backdrop:   "https://placehold.co/500x750/000000/ffffff?text=APPLE+TV%2B%0A%0ASilo%3A+T2",
			Overview:   "Serie disponible en streaming.",
			Lang:       "es-ES",
			IsEstreno:  true,
			Providers:  []string{"Apple TV+"},
			ExternalID: 114472,
		},
		{
			ID:         "ap-slow-3",
			Title:      "Slow Horses: T3",
			Year:       2024,
			Rating:     8.3,
			Duration:   "Serie",
			Kind:       model.Kind("series"),
			Genres:     []string{"Thriller"},
			Poster:     "https://placehold.co/500x750/000000/ffffff?text=APPLE+TV%2B%0A%0ASlow+Horses%3A+T3",
			Backdrop:   "https://placehold.co/500x750/000000/ffffff?text=APPLE+TV%2B%0A%0ASlow+Horses%3A+T3",
			Overview:   "Serie disponible en streaming.",
			Lang:       "es-ES",
			IsEstreno:  true,
			Providers:  []string{"Apple TV+"},
			ExternalID: 114472,
		},
		{
			ID:         "ap-dark-mat",
			Title:      "Dark Matter",
			Year:       2024,
			Rating:     7.7,
			Duration:   "Serie",
			Kind:       model.Kind("series"),
			Genres:     []string{"Sci-Fi", "Thriller"},
			Poster:     "https://placehold.co/500x750/000000/ffffff?text=APPLE+TV%2B%0A%0ADark+Matter",
			Backdrop:   "https://placehold.co/500x750/000000/ffffff?text=APPLE+TV%2B%0A%0ADark+Matter",
			Overview:   "Serie disponible en streaming.",
			Lang:       "es-ES",
			IsEstreno:  true,
			Providers:  []string{"Apple TV+"},
			ExternalID: 114472,
		},
		{
			ID:         "ap-monarch",
			Title:      "Monarch: Legacy",
			Year:       2024,
			Rating:     7.0,
			Duration:   "Serie",
			Kind:       model.Kind("series"),
			Genres:     []string{"Sci-Fi", "Acción"},
			Poster:     "https://placehold.co/500x750/000000/ffffff?text=APPLE+TV%2B%0A%0AMonarch%3A+Legacy",
			Backdrop:   "https://placehold.co/500x750/000000/ffffff?text=APPLE+TV%2B%0A%0AMonarch%3A+Legacy",
			Overview:   "Serie disponible en streaming.",
			Lang:       "es-ES",
			IsEstreno:  true,
			Providers:  []string{"Apple TV+"},
			ExternalID: 114472,
		},
		{
			ID:         "ap-morning-3",
			Title:      "The Morning Show: T3",
			Year:       2023,
			Rating:     8.3,
			Duration:   "Serie",
			Kind:       model.Kind("series"),
			Genres:     []string{"Drama"},
			Poster:     "https://placehold.co/500x750/000000/ffffff?text=APPLE+TV%2B%0A%0AThe+Morning+Show%3A+T3",
			Backdrop:   "https://placehold.co/500x750/000000/ffffff?text=APPLE+TV%2B%0A%0AThe+Morning+Show%3A+T3",
			Overview:   "Serie disponible en streaming.",
			Lang:       "es-ES",
			IsEstreno:  false,
			Providers:  []string{"Apple TV+"},
			ExternalID: 114472,
		},
		{
			ID:         "tr-killers",
			Title:      "Killers of the Flower M",
			Year:       2023,
			Rating:     7.6,
			Duration:   "3h 26m",
			Kind:       model.Kind("movie"),
			Genres:     []string{"Drama"},
			Poster:     "https://placehold.co/500x750/000000/ffffff?text=APPLE+TV%2B%0A%0AKillers+of+the+Flower+M",
			Backdrop:   "https://placehold.co/500x750/000000/ffffff?text=APPLE+TV%2B%0A%0AKillers+of+the+Flower+M",
			Overview:   "Película disponible en streaming.",
			Lang:       "es-ES",
			IsEstreno:  false,
			Providers:  []string{"Apple TV+"},
			ExternalID: 466420,
		},

		// ── DISNEY+ ──
		{
			ID:         "ds-andor-2",
			Title:      "Andor: T2",
			Year:       2025,
			Rating:     8.4,
			Duration:   "Serie",
			Kind:       model.Kind("series"),
			Genres:     []string{"Sci-Fi", "Drama"},
			Poster:     "https://placehold.co/500x750/0f1e3d/ffffff?text=DISNEY%2B%0A%0AAndor%3A+T2",
			Backdrop:   "https://placehold.co/500x750/0f1e3d/ffffff?text=DISNEY%2B%0A%0AAndor%3A+T2",
			Overview:   "Serie disponible en streaming.",
			Lang:       "es-ES",
			IsEstreno:  true,
			Providers:  []string{"Disney+"},
			ExternalID: 83867,
		},
		{
			ID:         "ds-skel",
			Title:      "Skeleton Crew",
			Year:       2025,
			Rating:     7.1,
			Duration:   "Serie",
			Kind:       model.Kind("series"),
			Genres:     []string{"Sci-Fi", "Aventura"},
			Poster:     "https://placehold.co/500x750/0f1e3d/ffffff?text=DISNEY%2B%0A%0ASkeleton+Crew",
			Backdrop:   "https://placehold.co/500x750/0f1e3d/ffffff?text=DISNEY%2B%0A%0ASkeleton+Crew",
			Overview:   "Serie disponible en streaming.",
			Lang:       "es-ES",
			IsEstreno:  true,
			Providers:  []string{"Disney+"},
			ExternalID: 114472,
		},
		{
			ID:         "mv-blade",
			Title:      "Blade",
			Year:       2025,
			Rating:     6.0,
			Duration:   "2h 05m",
			Kind:       model.Kind("movie"),
			Genres:     []string{"Acción", "Sci-Fi"},
			Poster:     "https://placehold.co/500x750/0f1e3d/ffffff?text=DISNEY%2B%0A%0ABlade",
			Backdrop:   "https://placehold.co/500x750/0f1e3d/ffffff?text=DISNEY%2B%0A%0ABlade",
			Overview:   "Película disponible en streaming.",
			Lang:       "es-ES",
			IsEstreno:  true,
			Providers:  []string{"Disney+"},
			ExternalID: 114472,
		},
		{
			ID:         "ds-xmen97",
			Title:      "X-Men '97",
			Year:       2024,
			Rating:     8.5,
			Duration:   "Serie",
			Kind:       model.Kind("series"),
			Genres:     []string{"Animación", "Acción"},
			Poster:     "https://placehold.co/500x750/0f1e3d/ffffff?text=DISNEY%2B%0A%0AX-Men+%2797",
			Backdrop:   "https://placehold.co/500x750/0f1e3d/ffffff?text=DISNEY%2B%0A%0AX-Men+%2797",
			Overview:   "Serie disponible en streaming.",
			Lang:       "es-ES",
			IsEstreno:  true,
			Providers:  []string{"Disney+"},
			ExternalID: 114472,
		},
		{
			ID:         "ds-agatha",
			Title:      "Agatha All Along",
			Year:       2024,
			Rating:     6.8,
			Duration:   "Serie",
			Kind:       model.Kind("series"),
			Genres:     []string{"Drama", "Sci-Fi"},
			Poster:     "https://placehold.co/500x750/0f1e3d/ffffff?text=DISNEY%2B%0A%0AAgatha+All+Along",
			Backdrop:   "https://placehold.co/500x750/0f1e3d/ffffff?text=DISNEY%2B%0A%0AAgatha+All+Along",
			Overview:   "Serie disponible en streaming.",
			Lang:       "es-ES",
			IsEstreno:  true,
			Providers:  []string{"Disney+"},
			ExternalID: 114472,
		},
		{
			ID:         "ds-io2",
			Title:      "Inside Out 2",
			Year:       2024,
			Rating:     7.7,
			Duration:   "1h 36m",
			Kind:       model.Kind("movie"),
			Genres:     []string{"Animación"},
			Poster:     "https://placehold.co/500x750/0f1e3d/ffffff?text=DISNEY%2B%0A%0AInside+Out+2",
			Backdrop:   "https://placehold.co/500x750/0f1e3d/ffffff?text=DISNEY%2B%0A%0AInside+Out+2",
			Overview:   "Película disponible en streaming.",
			Lang:       "es-ES",
			IsEstreno:  true,
			Providers:  []string{"Disney+"},
			ExternalID: 1022789,
		},
		{
			ID:         "ds-dpw",
			Title:      "Deadpool & Wolverine",
			Year:       2024,
			Rating:     7.7,
			Duration:   "2h 08m",
			Kind:       model.Kind("movie"),
			Genres:     []string{"Acción"},
			Poster:     "https://placehold.co/500x750/0f1e3d/ffffff?text=DISNEY%2B%0A%0ADeadpool+%26+Wolverine",
			Backdrop:   "https://placehold.co/500x750/0f1e3d/ffffff?text=DISNEY%2B%0A%0ADeadpool+%26+Wolverine",
			Overview:   "Película disponible en streaming.",
			Lang:       "es-ES",
			IsEstreno:  true,
			Providers:  []string{"Disney+"},
			ExternalID: 533535,
		},
		{
			ID:         "ds-moana2",
			Title:      "Moana 2",
			Year:       2024,
			Rating:     6.9,
			Duration:   "1h 40m",
			Kind:       model.Kind("movie"),
			Genres:     []string{"Animación"},
			Poster:     "https://placehold.co/500x750/0f1e3d/ffffff?text=DISNEY%2B%0A%0AMoana+2",
			Backdrop:   "https://placehold.co/500x750/0f1e3d/ffffff?text=DISNEY%2B%0A%0AMoana+2",
			Overview:   "Película disponible en streaming.",
			Lang:       "es-ES",
			IsEstreno:  true,
			Providers:  []string{"Disney+"},
			ExternalID: 124198,
		},
		{
			ID:         "ds-bluey",
			Title:      "Bluey: T3",
			Year:       2024,
			Rating:     9.0,
			Duration:   "Serie",
			Kind:       model.Kind("series"),
			Genres:     []string{"Animación", "Familia"},
			Poster:     "https://placehold.co/500x750/0f1e3d/ffffff?text=DISNEY%2B%0A%0ABluey%3A+T3",
			Backdrop:   "https://placehold.co/500x750/0f1e3d/ffffff?text=DISNEY%2B%0A%0ABluey%3A+T3",
			Overview:   "Serie disponible en streaming.",
			Lang:       "es-ES",
			IsEstreno:  true,
			Providers:  []string{"Disney+"},
			ExternalID: 114472,
		},

		// ── HBO MAX ──
		{
			ID:         "hb-hod-3",
			Title:      "House of the Dragon: T3",
			Year:       2026,
			Rating:     8.7,
			Duration:   "Serie",
			Kind:       model.Kind("series"),
			Genres:     []string{"Fantasía", "Drama"},
			Poster:     "https://placehold.co/500x750/9b51e0/ffffff?text=HBO+MAX%0A%0AHouse+of+the+Dragon%3A+T3",
			Backdrop:   "https://placehold.co/500x750/9b51e0/ffffff?text=HBO+MAX%0A%0AHouse+of+the+Dragon%3A+T3",
			Overview:   "Serie disponible en streaming.",
			Lang:       "es-ES",
			IsEstreno:  true,
			Providers:  []string{"HBO Max"},
			ExternalID: 94997,
		},
		{
			ID:         "hb-tlou-2",
			Title:      "The Last of Us: T2",
			Year:       2025,
			Rating:     8.9,
			Duration:   "Serie",
			Kind:       model.Kind("series"),
			Genres:     []string{"Drama", "Sci-Fi"},
			Poster:     "https://placehold.co/500x750/9b51e0/ffffff?text=HBO+MAX%0A%0AThe+Last+of+Us%3A+T2",
			Backdrop:   "https://placehold.co/500x750/9b51e0/ffffff?text=HBO+MAX%0A%0AThe+Last+of+Us%3A+T2",
			Overview:   "Serie disponible en streaming.",
			Lang:       "es-ES",
			IsEstreno:  true,
			Providers:  []string{"HBO Max"},
			ExternalID: 100088,
		},
		{
			ID:         "hb-wtl-3",
			Title:      "The White Lotus: T3",
			Year:       2025,
			Rating:     8.0,
			Duration:   "Serie",
			Kind:       model.Kind("series"),
			Genres:     []string{"Drama"},
			Poster:     "https://placehold.co/500x750/9b51e0/ffffff?text=HBO+MAX%0A%0AThe+White+Lotus%3A+T3",
			Backdrop:   "https://placehold.co/500x750/9b51e0/ffffff?text=HBO+MAX%0A%0AThe+White+Lotus%3A+T3",
			Overview:   "Serie disponible en streaming.",
			Lang:       "es-ES",
			IsEstreno:  true,
			Providers:  []string{"HBO Max"},
			ExternalID: 114472,
		},
		{
			ID:         "hb-dune-p",
			Title:      "Dune: Prophecy",
			Year:       2024,
			Rating:     7.4,
			Duration:   "Serie",
			Kind:       model.Kind("series"),
			Genres:     []string{"Sci-Fi"},
			Poster:     "https://placehold.co/500x750/9b51e0/ffffff?text=HBO+MAX%0A%0ADune%3A+Prophecy",
			Backdrop:   "https://placehold.co/500x750/9b51e0/ffffff?text=HBO+MAX%0A%0ADune%3A+Prophecy",
			Overview:   "Serie disponible en streaming.",
			Lang:       "es-ES",
			IsEstreno:  true,
			Providers:  []string{"HBO Max"},
			ExternalID: 114472,
		},
		{
			ID:         "hb-penguin",
			Title:      "The Penguin",
			Year:       2024,
			Rating:     8.7,
			Duration:   "Serie",
			Kind:       model.Kind("series"),
			Genres:     []string{"Drama", "Crimen"},
			Poster:     "https://placehold.co/500x750/9b51e0/ffffff?text=HBO+MAX%0A%0AThe+Penguin",
			Backdrop:   "https://placehold.co/500x750/9b51e0/ffffff?text=HBO+MAX%0A%0AThe+Penguin",
			Overview:   "Serie disponible en streaming.",
			Lang:       "es-ES",
			IsEstreno:  true,
			Providers:  []string{"HBO Max"},
			ExternalID: 114472,
		},
		{
			ID:         "hb-td-night",
			Title:      "True Detective: NC",
			Year:       2024,
			Rating:     7.4,
			Duration:   "Serie",
			Kind:       model.Kind("series"),
			Genres:     []string{"Misterio"},
			Poster:     "https://placehold.co/500x750/9b51e0/ffffff?text=HBO+MAX%0A%0ATrue+Detective%3A+NC",
			Backdrop:   "https://placehold.co/500x750/9b51e0/ffffff?text=HBO+MAX%0A%0ATrue+Detective%3A+NC",
			Overview:   "Serie disponible en streaming.",
			Lang:       "es-ES",
			IsEstreno:  true,
			Providers:  []string{"HBO Max"},
			ExternalID: 114472,
		},
		{
			ID:         "hb-dune2",
			Title:      "Dune: Part Two",
			Year:       2024,
			Rating:     8.5,
			Duration:   "2h 46m",
			Kind:       model.Kind("movie"),
			Genres:     []string{"Sci-Fi"},
			Poster:     "https://placehold.co/500x750/9b51e0/ffffff?text=HBO+MAX%0A%0ADune%3A+Part+Two",
			Backdrop:   "https://placehold.co/500x750/9b51e0/ffffff?text=HBO+MAX%0A%0ADune%3A+Part+Two",
			Overview:   "Película disponible en streaming.",
			Lang:       "es-ES",
			IsEstreno:  true,
			Providers:  []string{"HBO Max"},
			ExternalID: 693134,
		},
		{
			ID:         "hb-godzilla",
			Title:      "Godzilla x Kong",
			Year:       2024,
			Rating:     6.0,
			Duration:   "1h 55m",
			Kind:       model.Kind("movie"),
			Genres:     []string{"Acción"},
			Poster:     "https://placehold.co/500x750/9b51e0/ffffff?text=HBO+MAX%0A%0AGodzilla+x+Kong",
			Backdrop:   "https://placehold.co/500x750/9b51e0/ffffff?text=HBO+MAX%0A%0AGodzilla+x+Kong",
			Overview:   "Película disponible en streaming.",
			Lang:       "es-ES",
			IsEstreno:  true,
			Providers:  []string{"HBO Max"},
			ExternalID: 823464,
		},
		{
			ID:         "mv-dune2-mv",
			Title:      "Dune: Part Two",
			Year:       2024,
			Rating:     8.5,
			Duration:   "2h 46m",
			Kind:       model.Kind("movie"),
			Genres:     []string{"Sci-Fi"},
			Poster:     "https://placehold.co/500x750/9b51e0/ffffff?text=HBO+MAX%0A%0ADune%3A+Part+Two",
			Backdrop:   "https://placehold.co/500x750/9b51e0/ffffff?text=HBO+MAX%0A%0ADune%3A+Part+Two",
			Overview:   "Película disponible en streaming.",
			Lang:       "es-ES",
			IsEstreno:  true,
			Providers:  []string{"HBO Max"},
			ExternalID: 693134,
		},
		{
			ID:         "mv-civilwar",
			Title:      "Civil War",
			Year:       2024,
			Rating:     7.0,
			Duration:   "1h 49m",
			Kind:       model.Kind("movie"),
			Genres:     []string{"Acción"},
			Poster:     "https://placehold.co/500x750/9b51e0/ffffff?text=HBO+MAX%0A%0ACivil+War",
			Backdrop:   "https://placehold.co/500x750/9b51e0/ffffff?text=HBO+MAX%0A%0ACivil+War",
			Overview:   "Película disponible en streaming.",
			Lang:       "es-ES",
			IsEstreno:  true,
			Providers:  []string{"HBO Max"},
			ExternalID: 929590,
		},
		{
			ID:         "tr-oppen",
			Title:      "Oppenheimer",
			Year:       2023,
			Rating:     8.4,
			Duration:   "3h 00m",
			Kind:       model.Kind("movie"),
			Genres:     []string{"Drama"},
			Poster:     "https://placehold.co/500x750/9b51e0/ffffff?text=HBO+MAX%0A%0AOppenheimer",
			Backdrop:   "https://placehold.co/500x750/9b51e0/ffffff?text=HBO+MAX%0A%0AOppenheimer",
			Overview:   "Película disponible en streaming.",
			Lang:       "es-ES",
			IsEstreno:  false,
			Providers:  []string{"HBO Max"},
			ExternalID: 693134,
		},

		// ── HULU ──
		{
			ID:         "hu-bear-3",
			Title:      "The Bear: T3",
			Year:       2024,
			Rating:     8.6,
			Duration:   "Serie",
			Kind:       model.Kind("series"),
			Genres:     []string{"Drama"},
			Poster:     "https://placehold.co/500x750/1ce783/000000?text=HULU%0A%0AThe+Bear%3A+T3",
			Backdrop:   "https://placehold.co/500x750/1ce783/000000?text=HULU%0A%0AThe+Bear%3A+T3",
			Overview:   "Serie disponible en streaming.",
			Lang:       "es-ES",
			IsEstreno:  true,
			Providers:  []string{"Hulu"},
			ExternalID: 114472,
		},
		{
			ID:         "hu-omta-4",
			Title:      "Only Murders: T4",
			Year:       2024,
			Rating:     8.2,
			Duration:   "Serie",
			Kind:       model.Kind("series"),
			Genres:     []string{"Misterio", "Comedia"},
			Poster:     "https://placehold.co/500x750/1ce783/000000?text=HULU%0A%0AOnly+Murders%3A+T4",
			Backdrop:   "https://placehold.co/500x750/1ce783/000000?text=HULU%0A%0AOnly+Murders%3A+T4",
			Overview:   "Serie disponible en streaming.",
			Lang:       "es-ES",
			IsEstreno:  true,
			Providers:  []string{"Hulu"},
			ExternalID: 114472,
		},
		{
			ID:         "hu-shogun",
			Title:      "Shōgun",
			Year:       2024,
			Rating:     8.6,
			Duration:   "Serie",
			Kind:       model.Kind("series"),
			Genres:     []string{"Drama", "Aventura"},
			Poster:     "https://placehold.co/500x750/1ce783/000000?text=HULU%0A%0AShōgun",
			Backdrop:   "https://placehold.co/500x750/1ce783/000000?text=HULU%0A%0AShōgun",
			Overview:   "Serie disponible en streaming.",
			Lang:       "es-ES",
			IsEstreno:  true,
			Providers:  []string{"Hulu"},
			ExternalID: 114472,
		},
		{
			ID:         "hu-handmaid-5",
			Title:      "The Handmaid's Tale: T5",
			Year:       2024,
			Rating:     7.5,
			Duration:   "Serie",
			Kind:       model.Kind("series"),
			Genres:     []string{"Drama"},
			Poster:     "https://placehold.co/500x750/1ce783/000000?text=HULU%0A%0AThe+Handmaid%27s+Tale%3A+T5",
			Backdrop:   "https://placehold.co/500x750/1ce783/000000?text=HULU%0A%0AThe+Handmaid%27s+Tale%3A+T5",
			Overview:   "Serie disponible en streaming.",
			Lang:       "es-ES",
			IsEstreno:  true,
			Providers:  []string{"Hulu"},
			ExternalID: 114472,
		},
		{
			ID:         "hu-futurama",
			Title:      "Futurama (revival)",
			Year:       2023,
			Rating:     7.8,
			Duration:   "Serie",
			Kind:       model.Kind("series"),
			Genres:     []string{"Animación"},
			Poster:     "https://placehold.co/500x750/1ce783/000000?text=HULU%0A%0AFuturama+%28revival%29",
			Backdrop:   "https://placehold.co/500x750/1ce783/000000?text=HULU%0A%0AFuturama+%28revival%29",
			Overview:   "Serie disponible en streaming.",
			Lang:       "es-ES",
			IsEstreno:  false,
			Providers:  []string{"Hulu"},
			ExternalID: 114472,
		},
		{
			ID:         "tr-pbbt",
			Title:      "Poor Things",
			Year:       2023,
			Rating:     8.0,
			Duration:   "2h 21m",
			Kind:       model.Kind("movie"),
			Genres:     []string{"Comedia", "Sci-Fi"},
			Poster:     "https://placehold.co/500x750/1ce783/000000?text=HULU%0A%0APoor+Things",
			Backdrop:   "https://placehold.co/500x750/1ce783/000000?text=HULU%0A%0APoor+Things",
			Overview:   "Película disponible en streaming.",
			Lang:       "es-ES",
			IsEstreno:  false,
			Providers:  []string{"Hulu"},
			ExternalID: 792307,
		},

		// ── NETFLIX ──
		{
			ID:         "nf-wed-02",
			Title:      "Wednesday: T2",
			Year:       2025,
			Rating:     8.4,
			Duration:   "Serie",
			Kind:       model.Kind("series"),
			Genres:     []string{"Comedia", "Misterio"},
			Poster:     "https://placehold.co/500x750/e50914/ffffff?text=NETFLIX%0A%0AWednesday%3A+T2",
			Backdrop:   "https://placehold.co/500x750/e50914/ffffff?text=NETFLIX%0A%0AWednesday%3A+T2",
			Overview:   "Serie disponible en streaming.",
			Lang:       "es-ES",
			IsEstreno:  true,
			Providers:  []string{"Netflix"},
			ExternalID: 119051,
		},
		{
			ID:         "nf-sg-02",
			Title:      "Squid Game: T2",
			Year:       2024,
			Rating:     8.0,
			Duration:   "Serie",
			Kind:       model.Kind("series"),
			Genres:     []string{"Drama", "Thriller"},
			Poster:     "https://placehold.co/500x750/e50914/ffffff?text=NETFLIX%0A%0ASquid+Game%3A+T2",
			Backdrop:   "https://placehold.co/500x750/e50914/ffffff?text=NETFLIX%0A%0ASquid+Game%3A+T2",
			Overview:   "Serie disponible en streaming.",
			Lang:       "es-ES",
			IsEstreno:  true,
			Providers:  []string{"Netflix"},
			ExternalID: 93405,
		},
		{
			ID:         "nf-3body",
			Title:      "3 Body Problem",
			Year:       2024,
			Rating:     7.6,
			Duration:   "Serie",
			Kind:       model.Kind("series"),
			Genres:     []string{"Sci-Fi", "Drama"},
			Poster:     "https://placehold.co/500x750/e50914/ffffff?text=NETFLIX%0A%0A3+Body+Problem",
			Backdrop:   "https://placehold.co/500x750/e50914/ffffff?text=NETFLIX%0A%0A3+Body+Problem",
			Overview:   "Serie disponible en streaming.",
			Lang:       "es-ES",
			IsEstreno:  true,
			Providers:  []string{"Netflix"},
			ExternalID: 100088,
		},
		{
			ID:         "nf-bridg-3",
			Title:      "Bridgerton: T3",
			Year:       2024,
			Rating:     7.3,
			Duration:   "Serie",
			Kind:       model.Kind("series"),
			Genres:     []string{"Romance", "Drama"},
			Poster:     "https://placehold.co/500x750/e50914/ffffff?text=NETFLIX%0A%0ABridgerton%3A+T3",
			Backdrop:   "https://placehold.co/500x750/e50914/ffffff?text=NETFLIX%0A%0ABridgerton%3A+T3",
			Overview:   "Serie disponible en streaming.",
			Lang:       "es-ES",
			IsEstreno:  true,
			Providers:  []string{"Netflix"},
			ExternalID: 134374,
		},
		{
			ID:         "nf-rebel",
			Title:      "Rebel Moon (DC)",
			Year:       2024,
			Rating:     6.9,
			Duration:   "2h 15m",
			Kind:       model.Kind("movie"),
			Genres:     []string{"Sci-Fi", "Acción"},
			Poster:     "https://placehold.co/500x750/e50914/ffffff?text=NETFLIX%0A%0ARebel+Moon+%28DC%29",
			Backdrop:   "https://placehold.co/500x750/e50914/ffffff?text=NETFLIX%0A%0ARebel+Moon+%28DC%29",
			Overview:   "Película disponible en streaming.",
			Lang:       "es-ES",
			IsEstreno:  true,
			Providers:  []string{"Netflix"},
			ExternalID: 927342,
		},
		{
			ID:         "nf-damsel",
			Title:      "Damsel",
			Year:       2024,
			Rating:     6.4,
			Duration:   "1h 50m",
			Kind:       model.Kind("movie"),
			Genres:     []string{"Fantasía"},
			Poster:     "https://placehold.co/500x750/e50914/ffffff?text=NETFLIX%0A%0ADamsel",
			Backdrop:   "https://placehold.co/500x750/e50914/ffffff?text=NETFLIX%0A%0ADamsel",
			Overview:   "Película disponible en streaming.",
			Lang:       "es-ES",
			IsEstreno:  true,
			Providers:  []string{"Netflix"},
			ExternalID: 931642,
		},
		{
			ID:         "nf-extract2",
			Title:      "Extraction 2",
			Year:       2023,
			Rating:     7.0,
			Duration:   "2h 03m",
			Kind:       model.Kind("movie"),
			Genres:     []string{"Acción"},
			Poster:     "https://placehold.co/500x750/e50914/ffffff?text=NETFLIX%0A%0AExtraction+2",
			Backdrop:   "https://placehold.co/500x750/e50914/ffffff?text=NETFLIX%0A%0AExtraction+2",
			Overview:   "Película disponible en streaming.",
			Lang:       "es-ES",
			IsEstreno:  false,
			Providers:  []string{"Netflix"},
			ExternalID: 697843,
		},
		{
			ID:         "nf-glass",
			Title:      "Glass Onion",
			Year:       2022,
			Rating:     7.2,
			Duration:   "2h 19m",
			Kind:       model.Kind("movie"),
			Genres:     []string{"Misterio"},
			Poster:     "https://placehold.co/500x750/e50914/ffffff?text=NETFLIX%0A%0AGlass+Onion",
			Backdrop:   "https://placehold.co/500x750/e50914/ffffff?text=NETFLIX%0A%0AGlass+Onion",
			Overview:   "Película disponible en streaming.",
			Lang:       "es-ES",
			IsEstreno:  false,
			Providers:  []string{"Netflix"},
			ExternalID: 724088,
		},

		// ── PARAMOUNT+ ──
		{
			ID:         "pm-1923-2",
			Title:      "1923: T2",
			Year:       2025,
			Rating:     8.3,
			Duration:   "Serie",
			Kind:       model.Kind("series"),
			Genres:     []string{"Drama", "Western"},
			Poster:     "https://placehold.co/500x750/0064ff/ffffff?text=PARAMOUNT%2B%0A%0A1923%3A+T2",
			Backdrop:   "https://placehold.co/500x750/0064ff/ffffff?text=PARAMOUNT%2B%0A%0A1923%3A+T2",
			Overview:   "Serie disponible en streaming.",
			Lang:       "es-ES",
			IsEstreno:  true,
			Providers:  []string{"Paramount+"},
			ExternalID: 114472,
		},
		{
			ID:         "pm-lioness-2",
			Title:      "Lioness: T2",
			Year:       2024,
			Rating:     7.8,
			Duration:   "Serie",
			Kind:       model.Kind("series"),
			Genres:     []string{"Acción", "Drama"},
			Poster:     "https://placehold.co/500x750/0064ff/ffffff?text=PARAMOUNT%2B%0A%0ALioness%3A+T2",
			Backdrop:   "https://placehold.co/500x750/0064ff/ffffff?text=PARAMOUNT%2B%0A%0ALioness%3A+T2",
			Overview:   "Serie disponible en streaming.",
			Lang:       "es-ES",
			IsEstreno:  true,
			Providers:  []string{"Paramount+"},
			ExternalID: 114472,
		},
		{
			ID:         "pm-tulsa-2",
			Title:      "Tulsa King: T2",
			Year:       2024,
			Rating:     7.7,
			Duration:   "Serie",
			Kind:       model.Kind("series"),
			Genres:     []string{"Drama", "Comedia"},
			Poster:     "https://placehold.co/500x750/0064ff/ffffff?text=PARAMOUNT%2B%0A%0ATulsa+King%3A+T2",
			Backdrop:   "https://placehold.co/500x750/0064ff/ffffff?text=PARAMOUNT%2B%0A%0ATulsa+King%3A+T2",
			Overview:   "Serie disponible en streaming.",
			Lang:       "es-ES",
			IsEstreno:  true,
			Providers:  []string{"Paramount+"},
			ExternalID: 114472,
		},
		{
			ID:         "mv-ghostb",
			Title:      "Ghostbusters: FE",
			Year:       2024,
			Rating:     6.2,
			Duration:   "1h 55m",
			Kind:       model.Kind("movie"),
			Genres:     []string{"Comedia"},
			Poster:     "https://placehold.co/500x750/0064ff/ffffff?text=PARAMOUNT%2B%0A%0AGhostbusters%3A+FE",
			Backdrop:   "https://placehold.co/500x750/0064ff/ffffff?text=PARAMOUNT%2B%0A%0AGhostbusters%3A+FE",
			Overview:   "Película disponible en streaming.",
			Lang:       "es-ES",
			IsEstreno:  true,
			Providers:  []string{"Paramount+"},
			ExternalID: 823464,
		},
		{
			ID:         "pm-yel-5",
			Title:      "Yellowstone: T5P2",
			Year:       2023,
			Rating:     8.7,
			Duration:   "Serie",
			Kind:       model.Kind("series"),
			Genres:     []string{"Drama", "Western"},
			Poster:     "https://placehold.co/500x750/0064ff/ffffff?text=PARAMOUNT%2B%0A%0AYellowstone%3A+T5P2",
			Backdrop:   "https://placehold.co/500x750/0064ff/ffffff?text=PARAMOUNT%2B%0A%0AYellowstone%3A+T5P2",
			Overview:   "Serie disponible en streaming.",
			Lang:       "es-ES",
			IsEstreno:  false,
			Providers:  []string{"Paramount+"},
			ExternalID: 114472,
		},
		{
			ID:         "pm-mib-7",
			Title:      "MI: Dead Reckoning",
			Year:       2023,
			Rating:     7.7,
			Duration:   "2h 43m",
			Kind:       model.Kind("movie"),
			Genres:     []string{"Acción"},
			Poster:     "https://placehold.co/500x750/0064ff/ffffff?text=PARAMOUNT%2B%0A%0AMI%3A+Dead+Reckoning",
			Backdrop:   "https://placehold.co/500x750/0064ff/ffffff?text=PARAMOUNT%2B%0A%0AMI%3A+Dead+Reckoning",
			Overview:   "Película disponible en streaming.",
			Lang:       "es-ES",
			IsEstreno:  false,
			Providers:  []string{"Paramount+"},
			ExternalID: 575264,
		},
		{
			ID:         "pm-bebop",
			Title:      "Cowboy Bebop (cast)",
			Year:       2021,
			Rating:     6.7,
			Duration:   "Serie",
			Kind:       model.Kind("series"),
			Genres:     []string{"Sci-Fi"},
			Poster:     "https://placehold.co/500x750/0064ff/ffffff?text=PARAMOUNT%2B%0A%0ACowboy+Bebop+%28cast%29",
			Backdrop:   "https://placehold.co/500x750/0064ff/ffffff?text=PARAMOUNT%2B%0A%0ACowboy+Bebop+%28cast%29",
			Overview:   "Serie disponible en streaming.",
			Lang:       "es-ES",
			IsEstreno:  false,
			Providers:  []string{"Paramount+"},
			ExternalID: 114472,
		},

		// ── PRIME VIDEO ──
		{
			ID:         "pv-roP-2",
			Title:      "Rings of Power: T2",
			Year:       2024,
			Rating:     6.9,
			Duration:   "Serie",
			Kind:       model.Kind("series"),
			Genres:     []string{"Fantasía", "Drama"},
			Poster:     "https://placehold.co/500x750/1399ff/ffffff?text=PRIME+VIDEO%0A%0ARings+of+Power%3A+T2",
			Backdrop:   "https://placehold.co/500x750/1399ff/ffffff?text=PRIME+VIDEO%0A%0ARings+of+Power%3A+T2",
			Overview:   "Serie disponible en streaming.",
			Lang:       "es-ES",
			IsEstreno:  true,
			Providers:  []string{"Prime Video"},
			ExternalID: 114472,
		},
		{
			ID:         "pv-boys-4",
			Title:      "The Boys: T4",
			Year:       2024,
			Rating:     8.2,
			Duration:   "Serie",
			Kind:       model.Kind("series"),
			Genres:     []string{"Acción", "Drama"},
			Poster:     "https://placehold.co/500x750/1399ff/ffffff?text=PRIME+VIDEO%0A%0AThe+Boys%3A+T4",
			Backdrop:   "https://placehold.co/500x750/1399ff/ffffff?text=PRIME+VIDEO%0A%0AThe+Boys%3A+T4",
			Overview:   "Serie disponible en streaming.",
			Lang:       "es-ES",
			IsEstreno:  true,
			Providers:  []string{"Prime Video"},
			ExternalID: 114472,
		},
		{
			ID:         "pv-fallout",
			Title:      "Fallout",
			Year:       2024,
			Rating:     8.4,
			Duration:   "Serie",
			Kind:       model.Kind("series"),
			Genres:     []string{"Sci-Fi", "Acción"},
			Poster:     "https://placehold.co/500x750/1399ff/ffffff?text=PRIME+VIDEO%0A%0AFallout",
			Backdrop:   "https://placehold.co/500x750/1399ff/ffffff?text=PRIME+VIDEO%0A%0AFallout",
			Overview:   "Serie disponible en streaming.",
			Lang:       "es-ES",
			IsEstreno:  true,
			Providers:  []string{"Prime Video"},
			ExternalID: 114472,
		},
		{
			ID:         "pv-cross",
			Title:      "Cross",
			Year:       2024,
			Rating:     7.5,
			Duration:   "Serie",
			Kind:       model.Kind("series"),
			Genres:     []string{"Drama"},
			Poster:     "https://placehold.co/500x750/1399ff/ffffff?text=PRIME+VIDEO%0A%0ACross",
			Backdrop:   "https://placehold.co/500x750/1399ff/ffffff?text=PRIME+VIDEO%0A%0ACross",
			Overview:   "Serie disponible en streaming.",
			Lang:       "es-ES",
			IsEstreno:  true,
			Providers:  []string{"Prime Video"},
			ExternalID: 114472,
		},
		{
			ID:         "pv-reacher-2",
			Title:      "Reacher: T2",
			Year:       2023,
			Rating:     8.1,
			Duration:   "Serie",
			Kind:       model.Kind("series"),
			Genres:     []string{"Acción", "Crimen"},
			Poster:     "https://placehold.co/500x750/1399ff/ffffff?text=PRIME+VIDEO%0A%0AReacher%3A+T2",
			Backdrop:   "https://placehold.co/500x750/1399ff/ffffff?text=PRIME+VIDEO%0A%0AReacher%3A+T2",
			Overview:   "Serie disponible en streaming.",
			Lang:       "es-ES",
			IsEstreno:  false,
			Providers:  []string{"Prime Video"},
			ExternalID: 114472,
		},
		{
			ID:         "tr-past-lives",
			Title:      "Past Lives",
			Year:       2023,
			Rating:     7.9,
			Duration:   "1h 45m",
			Kind:       model.Kind("movie"),
			Genres:     []string{"Romance"},
			Poster:     "https://placehold.co/500x750/1399ff/ffffff?text=PRIME+VIDEO%0A%0APast+Lives",
			Backdrop:   "https://placehold.co/500x750/1399ff/ffffff?text=PRIME+VIDEO%0A%0APast+Lives",
			Overview:   "Película disponible en streaming.",
			Lang:       "es-ES",
			IsEstreno:  false,
			Providers:  []string{"Prime Video"},
			ExternalID: 666277,
		},
		{
			ID:         "tr-zone",
			Title:      "The Zone of Interest",
			Year:       2023,
			Rating:     7.4,
			Duration:   "1h 45m",
			Kind:       model.Kind("movie"),
			Genres:     []string{"Drama"},
			Poster:     "https://placehold.co/500x750/1399ff/ffffff?text=PRIME+VIDEO%0A%0AThe+Zone+of+Interest",
			Backdrop:   "https://placehold.co/500x750/1399ff/ffffff?text=PRIME+VIDEO%0A%0AThe+Zone+of+Interest",
			Overview:   "Película disponible en streaming.",
			Lang:       "es-ES",
			IsEstreno:  false,
			Providers:  []string{"Prime Video"},
			ExternalID: 458576,
		},
		{
			ID:         "tr-holdovers",
			Title:      "The Holdovers",
			Year:       2023,
			Rating:     7.9,
			Duration:   "2h 13m",
			Kind:       model.Kind("movie"),
			Genres:     []string{"Drama"},
			Poster:     "https://placehold.co/500x750/1399ff/ffffff?text=PRIME+VIDEO%0A%0AThe+Holdovers",
			Backdrop:   "https://placehold.co/500x750/1399ff/ffffff?text=PRIME+VIDEO%0A%0AThe+Holdovers",
			Overview:   "Película disponible en streaming.",
			Lang:       "es-ES",
			IsEstreno:  false,
			Providers:  []string{"Prime Video"},
			ExternalID: 507086,
		},
	}

	// Enriquece con TMDB: poster_path + backdrop_path reales.
	// Cache en disco para no pegarle a TMDB cada restart.
	enrichFromTMDB(items)

	// Si no se encontró en TMDB, fallback a SVG inline con brand.
	for i := range items {
		if strings.Contains(items[i].Poster, "placehold.co") || items[i].Poster == "" {
			items[i].Poster = posterSVG(items[i].Title, items[i].Providers)
			items[i].Backdrop = posterSVG(items[i].Title, items[i].Providers)
		}
	}
	return items
}

// TMDBPoster es lo que cacheamos por external_id.
type TMDBPoster struct {
	PosterPath   string `json:"poster_path"`
	BackdropPath string `json:"backdrop_path"`
}

// enrichFromTMDB consulta TMDB una vez por external_id y guarda poster/backdrop.
// Cache en /tmp/tmdb_cache.json — sobrevive restart.
func enrichFromTMDB(items []model.Media) {
	apiKey := os.Getenv("TMDB_API_KEY")
	if apiKey == "" {
		return
	}

	// Cargar cache previa.
	cacheFile := "/tmp/tmdb_cache.json"
	cache := map[string]TMDBPoster{}
	if data, err := os.ReadFile(cacheFile); err == nil {
		_ = json.Unmarshal(data, &cache)
	}

	client := &http.Client{Timeout: 8 * time.Second}
	needFlush := false

	for i := range items {
		if items[i].ExternalID == 0 {
			continue
		}
		key := fmt.Sprintf("%s-%d", items[i].Kind, items[i].ExternalID)
		if cached, ok := cache[key]; ok {
			if cached.PosterPath != "" {
				items[i].Poster = "https://image.tmdb.org/t/p/w500" + cached.PosterPath
			}
			if cached.BackdropPath != "" {
				items[i].Backdrop = "https://image.tmdb.org/t/p/w1280" + cached.BackdropPath
			}
			continue
		}

		endpoint := fmt.Sprintf("https://api.themoviedb.org/3/movie/%d", items[i].ExternalID)
		if items[i].Kind == model.Kind("series") {
			endpoint = fmt.Sprintf("https://api.themoviedb.org/3/tv/%d", items[i].ExternalID)
		}

		req, _ := http.NewRequest("GET", endpoint+"?api_key="+apiKey, nil)
		resp, err := client.Do(req)
		if err != nil {
			continue
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if resp.StatusCode != 200 {
			continue
		}

		var data struct {
			PosterPath   string `json:"poster_path"`
			BackdropPath string `json:"backdrop_path"`
		}
		if err := json.Unmarshal(body, &data); err != nil {
			continue
		}

		cache[key] = TMDBPoster{PosterPath: data.PosterPath, BackdropPath: data.BackdropPath}
		needFlush = true
		if data.PosterPath != "" {
			items[i].Poster = "https://image.tmdb.org/t/p/w500" + data.PosterPath
		}
		if data.BackdropPath != "" {
			items[i].Backdrop = "https://image.tmdb.org/t/p/w1280" + data.BackdropPath
		}
	}

	if needFlush {
		if data, err := json.MarshalIndent(cache, "", "  "); err == nil {
			os.WriteFile(cacheFile, data, 0644)
		}
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