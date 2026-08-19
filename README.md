# movie_api

REST API para `movie_spa`. Sirve metadata, catálogo y proxy de streams.

## Stack

- **Go 1.24**
- **Fiber v2** (HTTP framework sobre fasthttp)
- Almacenamiento **in-memory** con la misma seed del SPA — listo para cambiar por
  Postgres/SQLite/Jellyfin cuando se enchufe el data source real.

## Endpoints

| Método | Ruta                       | Descripción                              |
|--------|----------------------------|------------------------------------------|
| GET    | `/healthz`                 | Liveness probe                           |
| GET    | `/api/media/trending`      | Lista trending global                    |
| GET    | `/api/media/movies`        | Solo películas                           |
| GET    | `/api/media/series`        | Solo series                              |
| GET    | `/api/media/hero`          | Hero destacado del home                  |
| GET    | `/api/media/top10`         | Top 10 con ranking                       |
| GET    | `/api/media/:id`           | Detalle (cast, director, episodes…)      |
| GET    | `/api/media/:id/stream`    | Proxy de video (soporta HTTP Range)      |
| GET    | `/api/me/continue`         | “Continuar viendo” del usuario           |
| GET    | `/api/search?q=…`          | Búsqueda full-text sobre el catálogo     |
| POST   | `/api/jobs`                | Encola un job para `movie_worker`        |

## Comandos

```bash
go mod tidy
go run ./cmd/server       # http://localhost:8080
go build -o bin/server ./cmd/server
```

## Estructura

```
movie_api/
├── cmd/server/        # entrypoint
├── internal/
│   ├── config/        # env vars
│   ├── server/        # fiber app + middleware
│   ├── handlers/      # rutas HTTP
│   ├── store/         # interfaz + impl in-memory
│   └── model/         # tipos compartidos
└── pkg/client/        # cliente HTTP hacia movie_worker
```

## Conexión con `movie_spa`

`vite.config.js` ya hace `proxy /api → http://localhost:8080`. El SPA detecta el backend
vía el composable `useMedia` (cambiar `USE_MOCK = false` en `src/composables/useMedia.js`).

## Hand-off con `movie_worker`

Cuando el cliente pide `POST /api/jobs` con `{ "kind": "transcode", "media_id": "tt-001" }`,
el API reenvía a `movie_worker` por HTTP interno (`WORKER_URL`, default
`http://localhost:9090`) y devuelve el `job_id`. El worker procesa en background,
escribe el resultado a Postgres (compartido) o publica un evento.