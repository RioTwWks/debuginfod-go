package webapi

import (
	"compress/gzip"
	"io"
	"net/http"
	"strings"

	"github.com/your-username/debuginfod-go/internal/metrics"
)

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

// MetricsMiddleware учитывает HTTP-метрики для Zabbix (без Web UI).
func MetricsMiddleware(collector *metrics.Collector, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rw := &responseWriter{ResponseWriter: w}
		next.ServeHTTP(rw, r)
		if rw.status == 0 {
			rw.status = http.StatusOK
		}
		if shouldRecordHTTPMetric(r.URL.Path) {
			collector.RecordHTTP(rw.status, rw.bytes)
		}
	})
}

// shouldRecordHTTPMetric возвращает false для /ui/* — polling дашборда не должен раздувать счётчик API.
func shouldRecordHTTPMetric(path string) bool {
	return path != "/ui" && !strings.HasPrefix(path, "/ui/")
}

type gzipResponseWriter struct {
	io.Writer
	http.ResponseWriter
}

func (w gzipResponseWriter) Write(b []byte) (int, error) {
	return w.Writer.Write(b)
}

// GzipMiddleware сжимает ответы, если клиент поддерживает gzip.
func GzipMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
			next.ServeHTTP(w, r)
			return
		}
		w.Header().Set("Content-Encoding", "gzip")
		w.Header().Del("Content-Length")
		gz := gzip.NewWriter(w)
		defer gz.Close()
		next.ServeHTTP(gzipResponseWriter{Writer: gz, ResponseWriter: w}, r)
	})
}
