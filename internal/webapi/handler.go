package webapi

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/your-username/debuginfod-go/internal/federation"
	"github.com/your-username/debuginfod-go/internal/metrics"
	"github.com/your-username/debuginfod-go/internal/storage"
	"github.com/your-username/debuginfod-go/pkg/buildid"
	"github.com/your-username/debuginfod-go/pkg/elfsection"
)

// ServerOpts — зависимости HTTP-слоя.
type ServerOpts struct {
	Store            *storage.Storage
	MetadataMaxTime  time.Duration
	Federation       *federation.Client
	Metrics          *metrics.Collector
	ZabbixKey        string
	CacheBytes       func() int64
}

// Handler обслуживает HTTP-запросы протокола debuginfod.
type Handler struct {
	store      *storage.Storage
	federation *federation.Client
	metrics    *metrics.Collector
}

// NewHandler создаёт HTTP-обработчик.
func NewHandler(opts ServerOpts) *Handler {
	return &Handler{store: opts.Store, federation: opts.Federation, metrics: opts.Metrics}
}

// ServeHTTP маршрутизирует запросы buildid API.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	path := strings.TrimPrefix(r.URL.Path, "/")
	parts := strings.Split(path, "/")
	if len(parts) < 3 || parts[0] != "buildid" {
		http.NotFound(w, r)
		return
	}

	buildID := buildid.Normalize(parts[1])
	kind := parts[2]

	switch kind {
	case "debuginfo":
		h.serveArtifact(w, r, buildID, "debuginfo")
	case "executable":
		h.serveArtifact(w, r, buildID, "executable")
	case "source":
		if len(parts) < 4 {
			http.Error(w, "source path required", http.StatusBadRequest)
			return
		}
		sourcePath := "/" + strings.Join(parts[3:], "/")
		h.serveSource(w, r, buildID, sourcePath)
	case "section":
		if len(parts) < 4 {
			http.Error(w, "section name required", http.StatusBadRequest)
			return
		}
		h.serveSection(w, r, buildID, parts[3])
	default:
		http.NotFound(w, r)
	}
}

// MetadataHandler обрабатывает GET /metadata?key=...&value=...
func MetadataHandler(opts ServerOpts) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		key := r.URL.Query().Get("key")
		value := r.URL.Query().Get("value")
		if key == "" || value == "" {
			http.Error(w, "key and value query params required", http.StatusBadRequest)
			return
		}

		ctx := r.Context()
		if opts.MetadataMaxTime > 0 {
			var cancel context.CancelFunc
			ctx, cancel = context.WithTimeout(ctx, opts.MetadataMaxTime)
			defer cancel()
		}

		resp, err := opts.Store.SearchMetadata(ctx, key, value)
		if err != nil {
			slog.Error("SearchMetadata failed", "key", key, "value", value, "err", err)
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			slog.Error("encode metadata", "err", err)
		}
	}
}

func (h *Handler) serveArtifact(w http.ResponseWriter, r *http.Request, buildID, artifactType string) {
	filePath, err := h.store.GetArtifactPath(buildID, artifactType)
	if errors.Is(err, storage.ErrNotFound) {
		h.tryFederation(w, r)
		return
	}
	if err != nil {
		slog.Error("GetArtifactPath", "build_id", buildID, "type", artifactType, "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	http.ServeFile(w, r, filePath)
}

func (h *Handler) serveSource(w http.ResponseWriter, r *http.Request, buildID, sourcePath string) {
	filePath, err := h.store.GetSource(buildID, sourcePath)
	if errors.Is(err, storage.ErrNotFound) {
		h.tryFederation(w, r)
		return
	}
	if err != nil {
		slog.Error("GetSource", "build_id", buildID, "path", sourcePath, "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	http.ServeFile(w, r, filePath)
}

func (h *Handler) serveSection(w http.ResponseWriter, r *http.Request, buildID, sectionName string) {
	debuginfo, executable, err := h.store.GetArtifactPaths(buildID)
	if err != nil {
		slog.Error("GetArtifactPaths", "build_id", buildID, "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if debuginfo == "" && executable == "" {
		h.tryFederation(w, r)
		return
	}

	data, err := elfsection.ExtractFirst(debuginfo, executable, sectionName)
	if errors.Is(err, elfsection.ErrNotFound) {
		h.tryFederation(w, r)
		return
	}
	if err != nil {
		slog.Error("ExtractSection", "build_id", buildID, "section", sectionName, "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/octet-stream")
	_, _ = w.Write(data)
}

func (h *Handler) tryFederation(w http.ResponseWriter, r *http.Request) {
	if h.federation == nil || !h.federation.Enabled() {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	resp, err := h.federation.Fetch(r.URL.Path)
	if err != nil {
		if h.metrics != nil {
			h.metrics.RecordFederationMiss()
		}
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	defer resp.Body.Close()
	if h.metrics != nil {
		h.metrics.RecordFederationHit()
	}
	if _, err := federation.ProxyResponse(w, resp); err != nil {
		slog.Error("federation proxy", "err", err)
	}
}

// HealthHandler возвращает 200 OK для проверки живости сервиса.
func HealthHandler(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok\n"))
}

// NewMux создаёт ServeMux со всеми маршрутами debuginfod.
func NewMux(opts ServerOpts) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", HealthHandler)
	mux.HandleFunc("/metadata", MetadataHandler(opts))
	mux.HandleFunc("/zabbix", metrics.Handler(opts.Metrics, opts.Store, opts.CacheBytes, opts.ZabbixKey))
	mux.Handle("/buildid/", NewHandler(opts))

	var handler http.Handler = mux
	if opts.Metrics != nil {
		handler = MetricsMiddleware(opts.Metrics, handler)
	}
	handler = GzipMiddleware(handler)
	return handler
}
