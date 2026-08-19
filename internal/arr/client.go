// Package arr habla con Radarr/Sonarr desde el API.
// Hoy solo necesita proxy/lookup; más adelante podemos mover la
// orquestación entera (request → download → import) acá.
package arr

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

type Client struct {
	baseURL string
	apiKey  string
	http    *http.Client
}

func New(baseURL, apiKey string) *Client {
	return &Client{
		baseURL: baseURL,
		apiKey:  apiKey,
		http:    &http.Client{Timeout: 15 * time.Second},
	}
}

func (c *Client) Get(path string, out any) error {
	if c == nil || c.apiKey == "" {
		return fmt.Errorf("arr client not configured")
	}
	u := c.baseURL + path
	req, _ := http.NewRequest("GET", u, nil)
	req.Header.Set("X-Api-Key", c.apiKey)
	req.Header.Set("Accept", "application/json")
	r, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer r.Body.Close()
	if r.StatusCode >= 300 {
		body, _ := io.ReadAll(r.Body)
		return fmt.Errorf("arr GET %s: %s: %s", path, r.Status, body)
	}
	return json.NewDecoder(r.Body).Decode(out)
}

func (c *Client) Post(path string, body, out any) error {
	if c == nil || c.apiKey == "" {
		return fmt.Errorf("arr client not configured")
	}
	b, _ := json.Marshal(body)
	u := c.baseURL + path
	req, _ := http.NewRequest("POST", u, ioReader(b))
	req.Header.Set("X-Api-Key", c.apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	r, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer r.Body.Close()
	if r.StatusCode >= 300 {
		bb, _ := io.ReadAll(r.Body)
		return fmt.Errorf("arr POST %s: %s: %s", path, r.Status, bb)
	}
	if out == nil {
		return nil
	}
	return json.NewDecoder(r.Body).Decode(out)
}

func ioReader(b []byte) io.Reader { return &bytesReader{b: b} }

type bytesReader struct {
	b []byte
	i int
}

func (r *bytesReader) Read(p []byte) (int, error) {
	if r.i >= len(r.b) {
		return 0, io.EOF
	}
	n := copy(p, r.b[r.i:])
	r.i += n
	return n, nil
}

// helpers

func Q(s string) string { return url.QueryEscape(s) }