# syntax=docker/dockerfile:1.7
# ──────────────────────────────────────────────────────────────────────────────
# Stage 1: build con Go — binario estático (CGO=0)
# ──────────────────────────────────────────────────────────────────────────────
FROM golang:1.24-alpine AS build

WORKDIR /src

# Cache de módulos
COPY go.mod go.sum* ./
RUN go mod download

COPY . .

# Build estático, sin cgo, sin símbolos, con trimming y upx-friendly
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -trimpath -ldflags="-s -w" \
    -o /out/movie_api ./cmd/server

# ──────────────────────────────────────────────────────────────────────────────
# Stage 2: distroless static (~2MB, sin shell, sin libs)
# ──────────────────────────────────────────────────────────────────────────────
FROM gcr.io/distroless/static-debian12:nonroot AS runtime

LABEL org.opencontainers.image.title="movie_api" \
      org.opencontainers.image.source="https://example.local/movie_api"

COPY --from=build /out/movie_api /movie_api

ENV ADDR=:8080 \
    WORKER_URL=http://movie_worker:9090

EXPOSE 8080

USER nonroot:nonroot

ENTRYPOINT ["/movie_api"]