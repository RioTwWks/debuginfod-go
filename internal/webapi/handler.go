package webapi

import (
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

// ServeHTTP маршрутизирует запросы вида:
//   - /buildid/<id>/debuginfo
//   - /buildid/<id>/executable
//   - /buildid/<id>/source/<path>
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

func (h *Handler) serveArtifact(w http.ResponseWriter, r *http.Request, buildID, artifactType string) {
	filePath, err := h.storage.GetArtifact(buildID, artifactType)
	if errors.Is(err, storage.ErrNotFound) {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	if err != nil {
		log.Printf("GetArtifact(%s, %s): %v", buildID, artifactType, err)
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
