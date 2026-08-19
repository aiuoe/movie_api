package store

import "github.com/local/movie_api/internal/model"

// Store es la interfaz que el server consume. Cambiar la implementación es
// la única línea que hay que tocar para enchufar Postgres/SQLite/Jellyfin.
type Store interface {
	Seed() error

	Trending() []model.Media
	Movies() []model.Media
	Series() []model.Media
	Hero() (model.Hero, bool)
	Top10() []model.Media
	ByID(id string) (model.Media, bool)
	ContinueWatching() []model.ContinueWatching
	Search(q string) []model.Media
}