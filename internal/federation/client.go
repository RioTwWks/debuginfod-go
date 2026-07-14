package federation

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Client запрашивает артефакты у upstream debuginfod-серверов.
type Client struct {
	urls   []string
	client *http.Client
}

// New создаёт federation-клиент.
func New(urls []string, timeout time.Duration) *Client {
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	return &Client{
		urls:   urls,
		client: &http.Client{Timeout: timeout},
	}
}

// Enabled возвращает true, если настроены upstream URL.
func (c *Client) Enabled() bool {
	return c != nil && len(c.urls) > 0
}

// Fetch выполняет GET на upstream-серверах до первого 200 OK.
func (c *Client) Fetch(path string) (*http.Response, error) {
	if !c.Enabled() {
		return nil, fmt.Errorf("no upstream urls")
	}
	path = strings.TrimPrefix(path, "/")

	var lastErr error
	for _, base := range c.urls {
		base = strings.TrimRight(base, "/")
		url := base + "/" + path
		resp, err := c.client.Get(url)
		if err != nil {
			lastErr = err
			continue
		}
		if resp.StatusCode == http.StatusOK {
			return resp, nil
		}
		resp.Body.Close()
		lastErr = fmt.Errorf("upstream %s: status %d", base, resp.StatusCode)
	}
	return nil, lastErr
}

// ProxyResponse копирует upstream-ответ клиенту.
func ProxyResponse(w http.ResponseWriter, resp *http.Response) (int64, error) {
	if ct := resp.Header.Get("Content-Type"); ct != "" {
		w.Header().Set("Content-Type", ct)
	}
	if cl := resp.Header.Get("Content-Length"); cl != "" {
		w.Header().Set("Content-Length", cl)
	}
	w.WriteHeader(resp.StatusCode)
	return io.Copy(w, resp.Body)
}
