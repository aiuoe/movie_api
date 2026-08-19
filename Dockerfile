# syntax=docker/dockerfile:1.7
# ──────────────────────────────────────────────────────────────────────────────
# movie_api — Go + Fiber REST API
#
# Checkout del commit adentro (no COPY del build context).
# Args:
#   REPO   = aiuoe/movie_api
#   COMMIT = branch / tag / sha  (default: main)
# ──────────────────────────────────────────────────────────────────────────────

ARG REPO=aiuoe/movie_api
ARG COMMIT=main

FROM alpine/git:latest AS src
ARG REPO
ARG COMMIT
RUN apk add --no-cache bash && \
    git clone https://github.com/${REPO}.git /src && \
    cd /src && \
    git checkout ${COMMIT}

# ──────────────────────────────────────────────────────────────────────────────
# Stage 1: build con Go — binario estático
# ──────────────────────────────────────────────────────────────────────────────
FROM golang:1.24-alpine AS build

WORKDIR /src
COPY --from=src /src/go.mod /src/go.sum* ./
RUN go mod download

COPY --from=src /src/ ./
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -trimpath -ldflags="-s -w" \
    -o /out/movie_api ./cmd/server

# ──────────────────────────────────────────────────────────────────────────────
# Stage 2: distroless static (~2MB)
# ──────────────────────────────────────────────────────────────────────────────
FROM gcr.io/distroless/static-debian12:nonroot AS runtime

ARG COMMIT
ARG REPO
LABEL org.opencontainers.image.title="movie_api" \
      org.opencontainers.image.source="https://github.com/${REPO}" \
      org.opencontainers.image.revision="${COMMIT}" \
      org.opencontainers.image.licenses="MIT"

COPY --from=build /out/movie_api /movie_api

ENV ADDR=:8080 \
    WORKER_URL=http://movie_worker:9090

EXPOSE 8080

USER nonroot:nonroot

ENTRYPOINT ["/movie_api"]