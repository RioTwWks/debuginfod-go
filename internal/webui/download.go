package webui

import (
	"log/slog"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/your-username/debuginfod-go/internal/pathsafe"
)

func serveAttachment(w http.ResponseWriter, r *http.Request, path, filename string) {
	if filename == "" {
		filename = filepath.Base(path)
	}
	safe := strings.ReplaceAll(filename, `"`, "")
	w.Header().Set("Content-Disposition", `attachment; filename="`+safe+`"`)
	http.ServeFile(w, r, path)
}

func dedupDownloadHandler(opts Opts) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		id, err := parseDedupDownloadID(r.URL.Path)
		if err != nil {
			http.Error(w, "invalid dedup id", http.StatusBadRequest)
			return
		}

		df, err := opts.Store.GetDedupFileByID(id)
		if err != nil {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		if len(opts.AllowedRoots) > 0 {
			if err := pathsafe.AssertUnderRoots(df.FilePath, opts.AllowedRoots); err != nil {
				slog.Warn("webui dedup download forbidden", "id", id, "path", df.FilePath, "err", err)
				http.Error(w, "forbidden", http.StatusForbidden)
				return
			}
		}

		path := df.FilePath
		if opts.DedupRestorer != nil {
			restored, restoreErr := opts.DedupRestorer.RestoreToCache(opts.CacheDir, df.FilePath)
			if restoreErr != nil {
				slog.Error("webui dedup restore", "id", id, "path", df.FilePath, "err", restoreErr)
				http.Error(w, "restore error", http.StatusInternalServerError)
				return
			}
			path = restored
		}

		serveAttachment(w, r, path, df.Filename)
	}
}

func parseDedupDownloadID(path string) (int64, error) {
	const prefix = "/ui/api/download/dedup/"
	idStr := strings.TrimPrefix(path, prefix)
	idStr = strings.Trim(idStr, "/")
	return strconv.ParseInt(idStr, 10, 64)
}
