package webapi

import (
    "net/http"
    "path/filepath"
    "debuginfod-go/internal/storage"
)

type Handler struct {
    storage *storage.Storage
}

func NewHandler(storage *storage.Storage) *Handler {
    return &Handler{storage: storage}
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
    // Простейшая маршрутизация (можно использовать gorilla/mux или chi)
    // Пример: /buildid/abc123/debuginfo
    // ...
}

func (h *Handler) handleDebugInfo(w http.ResponseWriter, r *http.Request, buildID string) {
    path, err := h.storage.GetArtifact(buildID, "debuginfo")
    if err != nil {
        http.Error(w, "Internal error", http.StatusInternalServerError)
        return
    }
    if path == "" {
        http.Error(w, "Not found", http.StatusNotFound)
        return
    }
    http.ServeFile(w, r, path)
}
