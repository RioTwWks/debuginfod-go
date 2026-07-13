package webapi

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/your-username/debuginfod-go/internal/storage"
	"github.com/your-username/debuginfod-go/pkg/buildid"
	"github.com/your-username/debuginfod-go/pkg/elfsection"
)

// Handler обслуживает HTTP-запросы протокола debuginfod.
type Handler struct {
	storage *storage.Storage
}

// NewHandler создаёт HTTP-обработчик.
func NewHandler(store *storage.Storage) *Handler {
	return &Handler{storage: store}
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
func MetadataHandler(store *storage.Storage, maxTime time.Duration) http.HandlerFunc {
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
		if maxTime > 0 {
			var cancel context.CancelFunc
			ctx, cancel = context.WithTimeout(ctx, maxTime)
			defer cancel()
		}

		resp, err := store.SearchMetadata(ctx, key, value)
		if err != nil {
			log.Printf("SearchMetadata(%s, %s): %v", key, value, err)
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			log.Printf("encode metadata: %v", err)
		}
	}
}

func (h *Handler) serveArtifact(w http.ResponseWriter, r *http.Request, buildID, artifactType string) {
	filePath, err := h.storage.GetArtifactPath(buildID, artifactType)
	if errors.Is(err, storage.ErrNotFound) {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	if err != nil {
		log.Printf("GetArtifactPath(%s, %s): %v", buildID, artifactType, err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	http.ServeFile(w, r, filePath)
}

func (h *Handler) serveSource(w http.ResponseWriter, r *http.Request, buildID, sourcePath string) {
	filePath, err := h.storage.GetSource(buildID, sourcePath)
	if errors.Is(err, storage.ErrNotFound) {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	if err != nil {
		log.Printf("GetSource(%s, %s): %v", buildID, sourcePath, err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	http.ServeFile(w, r, filePath)
}

func (h *Handler) serveSection(w http.ResponseWriter, r *http.Request, buildID, sectionName string) {
	debuginfo, executable, err := h.storage.GetArtifactPaths(buildID)
	if err != nil {
		log.Printf("GetArtifactPaths(%s): %v", buildID, err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if debuginfo == "" && executable == "" {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	data, err := elfsection.ExtractFirst(debuginfo, executable, sectionName)
	if errors.Is(err, elfsection.ErrNotFound) {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	if err != nil {
		log.Printf("ExtractSection(%s, %s): %v", buildID, sectionName, err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/octet-stream")
	_, _ = w.Write(data)
}

// HealthHandler возвращает 200 OK для проверки живости сервиса.
func HealthHandler(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok\n"))
}

// NewMux создаёт ServeMux со всеми маршрутами debuginfod.
func NewMux(store *storage.Storage, metadataMaxTime time.Duration) *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", HealthHandler)
	mux.HandleFunc("/metadata", MetadataHandler(store, metadataMaxTime))
	mux.Handle("/buildid/", NewHandler(store))
	return mux
}
