package webapi

import (
	"embed"
	"io/fs"
	"net/http"
)

//go:embed openapi.yaml
var openAPIFS embed.FS

// OpenAPIHandler отдаёт спецификацию OpenAPI.
func OpenAPIHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	data, err := fs.ReadFile(openAPIFS, "openapi.yaml")
	if err != nil {
		http.Error(w, "openapi not found", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/yaml")
	_, _ = w.Write(data)
}
