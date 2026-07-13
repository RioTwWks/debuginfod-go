package webapi

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strings"

	"github.com/your-username/debuginfod-go/internal/storage"
	"github.com/your-username/debuginfod-go/pkg/buildid"
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
	default:
		http.NotFound(w, r)
	}
}

// MetadataHandler обрабатывает GET /metadata?key=...&value=...
func MetadataHandler(store *storage.Storage) http.HandlerFunc {
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

		resp, err := store.SearchMetadata(key, value)
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

// HealthHandler возвращает 200 OK для проверки живости сервиса.
func HealthHandler(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok\n"))
}
