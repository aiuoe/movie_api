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

// tmdbURL devuelve la URL del poster/background en el CDN de TMDB.
// Si pasás un poster_path real, la URL funciona sin auth. Para los
// placeholders usamos un placeholder local con el título encima.
func tmdbURL(path string, w int) string {
	if path == "" {
		return ""
	}
	return "https://image.tmdb.org/t/p/w" + itoa(w) + path
}

// seedMedia usa posters REALES de TMDB (URLs públicas del CDN).
// TMDB no requiere auth para servir imágenes — sí lo requiere para
// buscar/agregar títulos (eso lo hace Radarr/Sonarr por su lado).
func seedMedia() []model.Media {
	mk := func(id, title string, year int, rating float64, dur, kind, lang, posterPath, backdropPath string, genres []string, withStorage bool) model.Media {
		m := model.Media{
			ID:       id,
			Title:    title,
			Year:     year,
			Rating:   rating,
			Duration: dur,
			Kind:     model.Kind(kind),
			Genres:   genres,
			Poster:   tmdbURL(posterPath, 500),
			Backdrop: tmdbURL(backdropPath, 1280),
			Overview: "Lorem ipsum. Película / serie del catálogo local con metadata scrapeada automáticamente por el stack *arr.",
			Lang:     lang,
		}
		if withStorage {
			m.StorageKey = id + "/" + lang + "/source.mp4"
		}
		return m
	}
	return []model.Media{
		// ── DEMO REAL (storage_key → bucket) ─────────────────────────────
		mk("got-s01e01", "Big Buck Bunny — demo del pipeline", 2008, 8.0, "10m", "movie", "es-ES",
			"/qtH8Cv9CwxvOVeXyTz5eRlc2Ux.jpg",
			"/qm9Gbn9P6d0bl7K3Jz1CmH0Aa6T.jpg",
			[]string{"Animación", "Comedia"}, true),

		// ── Series ────────────────────────────────────────────────────────
		mk("got-001", "Juego de Tronos", 2011, 9.2, "Serie", "series", "es-ES",
			"/u3bZgnQ9v51KiDGu402J5tAyKwh.jpg", // GoT
			"/2OMB0ynKlyIenMJWI2Dy9IWT4Vs.jpg",
			[]string{"Fantasía", "Drama"}, false),
		mk("brk-001", "Breaking Bad", 2008, 9.5, "Serie", "series", "es-ES",
			"/ztkUQFLlC19CCMYHW73WiGsePXD.jpg",
			"/tsRy63Mu5cu02et9ZUKPSVUbt72.jpg",
			[]string{"Drama", "Thriller"}, false),
		mk("str-001", "Stranger Things", 2016, 8.7, "Serie", "series", "es-ES",
			"/49WJfeN0moxb9IPfGn8AIqMGskD.jpg",
			"/56v2KdpBlUM4oERiojjmF1GBbp6.jpg",
			[]string{"Sci-Fi", "Terror"}, false),
		mk("mnd-001", "The Mandalorian", 2019, 8.7, "Serie", "series", "es-ES",
			"/oZpSqux83v2aU9e5vM7yrfwLbu6.jpg",
			"/5V4xO0vzFbO8l4PpZ9OQjx9MFNY.jpg",
			[]string{"Sci-Fi", "Aventura"}, false),
		mk("wed-001", "The Witcher", 2019, 8.2, "Serie", "series", "es-ES",
			"/cZ0d3rtvXPVvuiX22sP9KDuNSPx.jpg",
			"/jBJWaqoSCiARWtfV0GlqmrcdxLw.jpg",
			[]string{"Fantasía"}, false),
		mk("dpr-001", "Dark", 2017, 8.8, "Serie", "series", "es-ES",
			"/apbrbWs8M9lyOpJYU5WXrpFdq1W.jpg",
			"/3lBDg3i6nn5R2NKFCJ6oKyUo2N5.jpg",
			[]string{"Sci-Fi", "Misterio"}, false),
		mk("hcd-001", "House of the Dragon", 2022, 8.4, "Serie", "series", "es-ES",
			"/7QMsOTMUswlwxJP0rTTZfmz2tX2.jpg",
			"/7vjjTpfJTbDAAsH8O1he9V9rjGA.jpg",
			[]string{"Fantasía"}, false),
		mk("thl-001", "The Last of Us", 2023, 8.7, "Serie", "series", "es-ES",
			"/uKvVjHNqB5VmOrdxqAt2F7J78ED.jpg",
			"/uDgy6hyPd82kOHh6I95FLtLnj6p.jpg",
			[]string{"Drama", "Sci-Fi"}, false),
		mk("sev-001", "Severance", 2022, 8.7, "Serie", "series", "es-ES",
			"/lXglP6j3w4WpckpDQRqCAUYHfPl.jpg",
			"/lXglP6j3w4WpckpDQRqCAUYHfPl.jpg",
			[]string{"Sci-Fi", "Thriller"}, false),

		// ── Películas ─────────────────────────────────────────────────────
		mk("inc-001", "Inception", 2010, 8.8, "2h 28m", "movie", "es-ES",
			"/9gk7adHYeDvHkCSEqAvQNLV5Uge.jpg",
			"/s3TBrRGB1iav7gFOCNx3H31MoES.jpg",
			[]string{"Sci-Fi", "Thriller"}, false),
		mk("int-001", "Interstellar", 2014, 8.7, "2h 49m", "movie", "es-ES",
			"/gEU2QniE6E77NI6lCU6MxlNBvIx.jpg",
			"/pbrkL804c8yAv3zBZR4QPEafpAR.jpg",
			[]string{"Sci-Fi", "Drama"}, false),
		mk("dun-001", "Dune", 2021, 8.0, "2h 35m", "movie", "es-ES",
			"/d5NXSklXo0qyIYkgV94XAgMIckC.jpg",
			"/iopYFB1b6Bh7FWZh3onQhph1sih.jpg",
			[]string{"Sci-Fi", "Aventura"}, false),
		mk("opn-001", "Oppenheimer", 2023, 8.4, "3h 00m", "movie", "es-ES",
			"/8Gxv8gSFCU0XGDykEGv7zR1n2ua.jpg",
			"/fm6KqXpk3M2HVveHwCrBSSBaO0V.jpg",
			[]string{"Drama", "Historia"}, false),
		mk("bar-001", "Barbie", 2023, 6.9, "1h 54m", "movie", "es-ES",
			"/iuFNMS8U5R6TMdB7uH4Wb8ukyH7.jpg",
			"/nHf61U7DFmdy84ii3GpXOAgkSty.jpg",
			[]string{"Comedia"}, false),
		mk("top-001", "Top Gun: Maverick", 2022, 8.3, "2h 11m", "movie", "es-ES",
			"/62HCnUTziyWcpDaBO2i1DX17ljH.jpg",
			"/odJ4hx6g6vFxuz3HtLsN6NdMbb3.jpg",
			[]string{"Acción"}, false),
		mk("jok-001", "Joker", 2019, 8.4, "2h 02m", "movie", "es-ES",
			"/udDclJoHjfjb8Ekgsd4FDteOkCU.jpg",
			"/n6bUvigpRFqSwmPp1m2YADdbRBc.jpg",
			[]string{"Drama", "Thriller"}, false),
		mk("par-001", "Parasite", 2019, 8.5, "2h 12m", "movie", "es-ES",
			"/7IiTTgloJzvGI1TAYymCfbfl3vT.jpg",
			"/TU9NIjwzjoKPwQHoHshkFcQUCG.jpg",
			[]string{"Thriller", "Drama"}, false),
		mk("avt-001", "Avengers: Endgame", 2019, 8.4, "3h 01m", "movie", "es-ES",
			"/or06FN3Dka5tukK1e9sl16pB3iy.jpg",
			"/orjiB3oUIsyz60hoEqkiGpy5CeO.jpg",
			[]string{"Acción", "Sci-Fi"}, false),
		mk("shn-001", "The Shawshank Redemption", 1994, 9.3, "2h 22m", "movie", "es-ES",
			"/q6y0Go1tsGEsmtFryDOJo3dEmqu.jpg",
			"/kXfqcdQKsToO0OUXHcrrNxhDBzM.jpg",
			[]string{"Drama"}, false),
		mk("gmf-001", "The Godfather", 1972, 9.2, "2h 55m", "movie", "es-ES",
			"/3bhkrj58Vtu7enYsRolD1fZdja1.jpg",
			"/tmU7GeKVybMWFButWEGl2M4GeiP.jpg",
			[]string{"Drama", "Crimen"}, false),
		mk("dpk-001", "Pulp Fiction", 1994, 8.9, "2h 34m", "movie", "es-ES",
			"/d5iIlFn5s0ImszYzBPb8JPIfbXD.jpg",
			"/suaEOtk1N1sgg2MTM7oZd2cfVp3.jpg",
			[]string{"Drama", "Crimen"}, false),
		mk("fig-001", "Fight Club", 1999, 8.8, "2h 19m", "movie", "es-ES",
			"/pB8BM7pdSp6B6Ih7QZ4DrQ3PmJK.jpg",
			"/52AfXWuXCHCz3mOzHWoXzfLTauR.jpg",
			[]string{"Drama", "Thriller"}, false),
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