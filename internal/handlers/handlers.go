package handlers

import (
	"github.com/local/movie_api/internal/config"
	"github.com/local/movie_api/internal/store"
)

// Handlers agrupa dependencias inyectadas. Mantener el struct chiquito
// hace que añadir un nuevo endpoint sea mecánico.
type Handlers struct {
	cfg config.Config
	st  store.Store
}

func New(cfg config.Config, st store.Store) *Handlers {
	return &Handlers{cfg: cfg, st: st}
}