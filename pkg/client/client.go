// Package client expone un cliente HTTP para hablar con movie_worker.
// Mantener este cliente separado evita que la lógica de “cómo hablo con
// el worker” se filtre en los handlers.
package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/local/movie_api/internal/model"
)

type Worker struct {
	baseURL string
	http    *http.Client
}

func New(baseURL string) *Worker {
	return &Worker{
		baseURL: baseURL,
		http: &http.Client{
			Timeout: 5 * time.Second,
		},
	}
}

// Submit envía un job al worker y devuelve el ID asignado por el worker.
// Si el worker está caído devuelve error — el caller decide si reintentar.
func (w *Worker) Submit(req model.JobRequest) (string, error) {
	body, _ := json.Marshal(req)
	resp, err := w.http.Post(w.baseURL+"/jobs", "application/json", bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("worker unreachable: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return "", fmt.Errorf("worker returned %s", resp.Status)
	}
	var out model.JobResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", err
	}
	return out.JobID, nil
}

// NotifyStreamReady le avisa al worker que hay un nuevo id listo para catalogar.
// Es fire-and-forget — no bloqueamos la respuesta del cliente si falla.
func (w *Worker) NotifyStreamReady(mediaID string) error {
	body, _ := json.Marshal(map[string]string{"media_id": mediaID})
	resp, err := w.http.Post(w.baseURL+"/streams/notify", "application/json", bytes.NewReader(body))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return nil
}

// Presign pide al worker una URL presigned para `key`, válida por `ttl`.
// La URL la emite el worker usando sus credenciales de MinIO/S3.
func (w *Worker) Presign(key string, ttl time.Duration) (string, error) {
	body, _ := json.Marshal(map[string]any{
		"key":      key,
		"ttl_secs": int(ttl.Seconds()),
	})
	resp, err := w.http.Post(w.baseURL+"/storage/presign", "application/json", bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("worker unreachable: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return "", fmt.Errorf("presign %s: %s", key, resp.Status)
	}
	var out struct {
		URL string `json:"url"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", err
	}
	return out.URL, nil
}