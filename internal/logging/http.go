package logging

import (
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"net/http"
	"strings"
	"time"
)

// RequestMiddleware логирует HTTP-запросы на уровне DEBUG.
// Пропускает health/readiness и polling Web UI.
func RequestMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !slog.Default().Enabled(r.Context(), slog.LevelDebug) {
			next.ServeHTTP(w, r)
			return
		}
		if !shouldLogHTTPRequest(r.URL.Path) {
			next.ServeHTTP(w, r)
			return
		}

		requestID := strings.TrimSpace(r.Header.Get("X-Request-ID"))
		if requestID == "" {
			requestID = newRequestID()
		}
		w.Header().Set("X-Request-ID", requestID)

		start := time.Now()
		rw := &responseWriter{ResponseWriter: w}
		ctx := WithContext(r.Context(), requestID)
		next.ServeHTTP(rw, r.WithContext(ctx))

		status := rw.status
		if status == 0 {
			status = http.StatusOK
		}
		LoggerFromContext(ctx, "http").Debug("request",
			"method", r.Method,
			"path", r.URL.Path,
			"query", r.URL.RawQuery,
			"status", status,
			"bytes", rw.bytes,
			"duration", time.Since(start),
			"remote", r.RemoteAddr,
			"user_agent", r.UserAgent(),
		)
	})
}

func shouldLogHTTPRequest(path string) bool {
	switch path {
	case "/healthz", "/readyz":
		return false
	}
	return path != "/ui" && !strings.HasPrefix(path, "/ui/")
}

type responseWriter struct {
	http.ResponseWriter
	status int
	bytes  int64
}

func (w *responseWriter) WriteHeader(status int) {
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

func (w *responseWriter) Write(p []byte) (int, error) {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	n, err := w.ResponseWriter.Write(p)
	w.bytes += int64(n)
	return n, err
}

func newRequestID() string {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return hex.EncodeToString([]byte(time.Now().Format("150405.000000")))
	}
	return hex.EncodeToString(b[:])
}
