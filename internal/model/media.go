package model

// Kind clasifica el contenido. Mantener en sync con el SPA.
type Kind string

const (
	KindMovie  Kind = "movie"
	KindSeries Kind = "series"
)

// Media es la unidad base de catálogo. Equivalente al `item` que consume el SPA.
type Media struct {
	ID       string   `json:"id"`
	Title    string   `json:"title"`
	Year     int      `json:"year"`
	Rating   float64  `json:"rating"`
	Duration string   `json:"duration"`
	Kind     Kind     `json:"kind"`
	Genres   []string `json:"genres"`
	Poster   string   `json:"poster"`
	Backdrop string   `json:"backdrop"`
	Overview string   `json:"overview"`
	Source   string   `json:"source,omitempty"` // URL interna del stream (legacy)

	// Storage path dentro del bucket (nuevo). El API lo usa para generar
	// una presigned URL vía movie_worker.
	StorageKey string `json:"storage_key,omitempty"`
	Lang       string `json:"lang,omitempty"` // "es-ES", "en-US"…

	// Solo series
	Seasons  int                    `json:"seasons,omitempty"`
	Episodes map[string][]Episode   `json:"episodes,omitempty"`

	// Detalle
	Cast     []string `json:"cast,omitempty"`
	Director string   `json:"director,omitempty"`
	Tagline  string   `json:"tagline,omitempty"`
	Similar  []string `json:"similar,omitempty"`
}

type Episode struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	Duration string `json:"duration"`
	Thumb    string `json:"thumb"`
	Overview string `json:"overview"`
}

// ContinueWatching une un Media con metadata de progreso del usuario.
type ContinueWatching struct {
	Media
	Progress float64 `json:"progress"`
	Episode  string  `json:"episode"`
}

// Hero es el item destacado en el home (un Media con tagline).
type Hero struct {
	Media
	Tagline string `json:"tagline"`
}

// JobRequest es el payload para encolar trabajo en movie_worker.
// `Notes` se serializa como JSON y se pasa como args al job.
type JobRequest struct {
	Kind    string `json:"kind"`    // "probe" | "transcode" | "scrape" | "ingest" | "download" | "upload"
	MediaID string `json:"media_id"`
	Notes   string `json:"notes,omitempty"` // JSON con media_id, source_url, lang, etc.
}

type JobResponse struct {
	JobID  string `json:"job_id"`
	Status string `json:"status"`
}