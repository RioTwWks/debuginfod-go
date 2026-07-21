package webui

import (
	"context"
	"embed"
	"encoding/json"
	"io/fs"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/your-username/debuginfod-go/internal/metrics"
	"github.com/your-username/debuginfod-go/internal/storage"
)

//go:embed static/*
var staticFiles embed.FS

// ScanTrigger запрашивает внеочередной scan индексатора.
type ScanTrigger interface {
	Trigger()
}

// Opts — зависимости Web UI.
type Opts struct {
	Store        *storage.Storage
	Metrics      *metrics.Collector
	CacheBytes   func() int64
	ScanPaths    []string
	DedupEnabled bool
	ScanEnabled  bool
	ScanTrigger  ScanTrigger
}

// StatsResponse — JSON для панели статистики.
type StatsResponse struct {
	UptimeSeconds       int64  `json:"uptime_seconds"`
	ArtifactsTotal      int64  `json:"artifacts_total"`
	ArtifactsExecutable int64  `json:"artifacts_executable"`
	ArtifactsDebuginfo  int64  `json:"artifacts_debuginfo"`
	SourcesTotal        int64  `json:"sources_total"`
	ScannedFilesTotal   int64  `json:"scanned_files_total"`
	LastScanDurationMs  int64  `json:"last_scan_duration_ms"`
	LastScanIndexed     int    `json:"last_scan_indexed"`
	LastScanSkipped     int    `json:"last_scan_skipped"`
	LastScanErrors      int    `json:"last_scan_errors"`
	LastScanFinishedAt  string `json:"last_scan_finished_at,omitempty"`
	HTTPRequestsTotal   uint64 `json:"http_requests_total"`
	CacheBytes          int64   `json:"cache_bytes"`
	IndexBytesOnDisk    int64   `json:"index_bytes_on_disk"`
	ScanEnabled         bool    `json:"scan_enabled"`
	DedupEnabled        bool    `json:"dedup_enabled"`
	DedupBytesSaved     int64   `json:"dedup_bytes_saved"`
	DedupSavedPercent   float64 `json:"dedup_saved_percent"`
}

// SearchResponse — JSON результатов поиска.
type SearchResponse struct {
	Key        string                   `json:"key,omitempty"`
	Query      string                   `json:"query,omitempty"`
	Value      string                   `json:"value,omitempty"`
	Results    []storage.ArtifactRecord `json:"results,omitempty"`
	Grouped    []storage.UIGroupedArtifact `json:"grouped,omitempty"`
	Count      int                      `json:"count"`
	Complete   bool                     `json:"complete"`
	NextOffset int                      `json:"next_offset,omitempty"`
}

// ScansResponse — история scan/dedup для вкладки «Сканирования».
type ScansResponse struct {
	IndexSummary   storage.IndexSummary         `json:"index_summary"`
	IndexScans     []storage.ScanRunRecord      `json:"index_scans"`
	DedupRuns      []storage.DedupRunRecord     `json:"dedup_runs"`
	DedupTotals    storage.DedupStorageTotals   `json:"dedup_totals"`
	DedupByProject []storage.DedupProjectTotals `json:"dedup_by_project"`
	DedupEnabled   bool                         `json:"dedup_enabled"`
}

// BrowseResponse — дерево .debug для Web UI.
type BrowseResponse struct {
	Query    string               `json:"query,omitempty"`
	Projects []storage.UITreeNode `json:"projects"`
	Count    int                  `json:"count"`
	Limit    int                  `json:"limit"`
	Complete bool                 `json:"complete"`
}

// Register добавляет маршруты Web UI в mux.
func Register(mux *http.ServeMux, opts Opts) {
	static, err := fs.Sub(staticFiles, "static")
	if err != nil {
		slog.Error("webui static fs", "err", err)
		return
	}

	mux.HandleFunc("/ui", redirectUI)
	mux.HandleFunc("/ui/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/ui/" || r.URL.Path == "/ui/index.html" {
			serveIndex(w, static)
			return
		}
		if strings.HasPrefix(r.URL.Path, "/ui/static/") {
			http.StripPrefix("/ui/static/", http.FileServer(http.FS(static))).ServeHTTP(w, r)
			return
		}
		http.NotFound(w, r)
	})
	mux.HandleFunc("/ui/api/stats", statsHandler(opts))
	mux.HandleFunc("/ui/api/browse", browseHandler(opts))
	mux.HandleFunc("/ui/api/search", searchHandler(opts))
	mux.HandleFunc("/ui/api/scans", scansHandler(opts))
	mux.HandleFunc("/ui/api/rescan", rescanHandler(opts))
}

func redirectUI(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, "/ui/", http.StatusMovedPermanently)
}

func serveIndex(w http.ResponseWriter, static fs.FS) {
	data, err := fs.ReadFile(static, "index.html")
	if err != nil {
		http.Error(w, "index not found", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(data)
}

func statsHandler(opts Opts) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		dbStats, err := opts.Store.Stats()
		if err != nil {
			slog.Error("webui stats", "err", err)
			http.Error(w, "stats error", http.StatusInternalServerError)
			return
		}

		scan := opts.Metrics.LastScan()
		resp := StatsResponse{
			UptimeSeconds:       int64(opts.Metrics.Uptime().Seconds()),
			ArtifactsTotal:      dbStats.ArtifactsTotal,
			ArtifactsExecutable: dbStats.ArtifactsExecutable,
			ArtifactsDebuginfo:  dbStats.ArtifactsDebuginfo,
			SourcesTotal:        dbStats.SourcesTotal,
			ScannedFilesTotal:   dbStats.ScannedFilesTotal,
			LastScanDurationMs:  scan.Duration.Milliseconds(),
			LastScanIndexed:     scan.Indexed,
			LastScanSkipped:     scan.Skipped,
			LastScanErrors:      scan.Errors,
			HTTPRequestsTotal:   opts.Metrics.HTTPRequests(),
			ScanEnabled:         opts.ScanEnabled,
			DedupEnabled:        opts.DedupEnabled,
		}
		if !scan.Finished.IsZero() {
			resp.LastScanFinishedAt = scan.Finished.UTC().Format(time.RFC3339)
		}
		if opts.CacheBytes != nil {
			resp.CacheBytes = opts.CacheBytes()
		}
		if summary, err := opts.Store.IndexSummary(); err != nil {
			slog.Warn("webui stats index summary", "err", err)
		} else {
			resp.IndexBytesOnDisk = summary.BytesOnDisk
		}
		if opts.DedupEnabled {
			if totals, err := opts.Store.DedupStorageTotals(); err != nil {
				slog.Warn("webui stats dedup totals", "err", err)
			} else {
				resp.DedupBytesSaved = totals.BytesSaved
				resp.DedupSavedPercent = totals.SavedPercent
			}
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}
}

func browseHandler(opts Opts) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		query := strings.TrimSpace(r.URL.Query().Get("q"))
		limit := 2000
		if raw := r.URL.Query().Get("limit"); raw != "" {
			if n, err := strconv.Atoi(raw); err == nil && n > 0 {
				limit = n
			}
		}

		ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
		defer cancel()

		records, err := opts.Store.SearchDebugFilesForUI(ctx, opts.ScanPaths, query, limit)
		if err != nil {
			slog.Error("webui browse", "query", query, "err", err)
			http.Error(w, "browse error", http.StatusInternalServerError)
			return
		}
		for i := range records {
			storage.EnrichArtifactComment(&records[i])
		}

		projects := storage.BuildUITree(opts.ScanPaths, records)
		complete := len(records) < limit

		resp := BrowseResponse{
			Query:    query,
			Projects: projects,
			Count:    len(records),
			Limit:    limit,
			Complete: complete,
		}
		if resp.Projects == nil {
			resp.Projects = []storage.UITreeNode{}
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}
}

func searchHandler(opts Opts) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		key := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("key")))
		if key == "" {
			key = "buildid"
		}

		limit := 50
		if raw := r.URL.Query().Get("limit"); raw != "" {
			if n, err := strconv.Atoi(raw); err == nil {
				limit = n
			}
		}

		offset := 0
		if raw := r.URL.Query().Get("offset"); raw != "" {
			if n, err := strconv.Atoi(raw); err == nil && n >= 0 {
				offset = n
			}
		}

		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()

		var resp SearchResponse

		switch key {
		case "buildid":
			query := r.URL.Query().Get("q")
			grouped, err := opts.Store.SearchBuildIDGroupedForUI(ctx, query, limit, opts.ScanPaths)
			if err != nil {
				slog.Error("webui search buildid", "query", query, "err", err)
				http.Error(w, "search error", http.StatusInternalServerError)
				return
			}
			resp = SearchResponse{
				Key:      key,
				Query:    query,
				Grouped:  grouped,
				Count:    len(grouped),
				Complete: true,
			}
		case "path", "name":
			value := strings.TrimSpace(r.URL.Query().Get("value"))
			if value == "" {
				value = strings.TrimSpace(r.URL.Query().Get("q"))
			}
			var meta storage.MetadataResponse
			var err error
			if key == "path" {
				meta, err = opts.Store.SearchPathForUI(ctx, opts.ScanPaths, value, offset, limit)
			} else {
				if value == "" {
					http.Error(w, "value required for name search", http.StatusBadRequest)
					return
				}
				meta, err = opts.Store.SearchNameForUI(ctx, opts.ScanPaths, value, offset, limit)
			}
			if err != nil {
				slog.Error("webui search", "key", key, "value", value, "err", err)
				http.Error(w, "search error", http.StatusInternalServerError)
				return
			}
			enrichFlatSearchResults(ctx, opts, meta.Results)
			resp = SearchResponse{
				Key:        key,
				Value:      value,
				Results:    meta.Results,
				Count:      len(meta.Results),
				Complete:   meta.Complete,
				NextOffset: meta.NextOffset,
			}
		case "glob", "file":
			value := strings.TrimSpace(r.URL.Query().Get("value"))
			if value == "" {
				value = strings.TrimSpace(r.URL.Query().Get("q"))
			}
			if value == "" {
				http.Error(w, "value required for "+key+" search", http.StatusBadRequest)
				return
			}
			meta, err := opts.Store.SearchMetadataQuery(ctx, storage.MetadataQuery{
				Key:    key,
				Value:  value,
				Offset: offset,
				Limit:  limit,
			})
			if err != nil {
				slog.Error("webui search metadata", "key", key, "value", value, "err", err)
				http.Error(w, "search error", http.StatusInternalServerError)
				return
			}
			for i := range meta.Results {
				storage.EnrichArtifactRecord(&meta.Results[i], opts.ScanPaths)
			}
			enrichFlatSearchResults(ctx, opts, meta.Results)
			resp = SearchResponse{
				Key:        key,
				Value:      value,
				Results:    meta.Results,
				Count:      len(meta.Results),
				Complete:   meta.Complete,
				NextOffset: meta.NextOffset,
			}
		default:
			http.Error(w, "unsupported search key: "+key, http.StatusBadRequest)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}
}

func scansHandler(opts Opts) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		limit := 50
		if raw := r.URL.Query().Get("limit"); raw != "" {
			if n, err := strconv.Atoi(raw); err == nil && n > 0 {
				limit = n
			}
		}
		indexScans, err := opts.Store.ListScanRuns(limit)
		if err != nil {
			slog.Error("webui list scan runs", "err", err)
			http.Error(w, "scans error", http.StatusInternalServerError)
			return
		}
		dedupRuns, err := opts.Store.ListDedupRuns(limit)
		if err != nil {
			slog.Error("webui list dedup runs", "err", err)
			http.Error(w, "scans error", http.StatusInternalServerError)
			return
		}
		totals, err := opts.Store.DedupStorageTotals()
		if err != nil {
			slog.Warn("webui dedup totals", "err", err)
		}
		byProject, err := opts.Store.DedupTotalsByProject()
		if err != nil {
			slog.Warn("webui dedup by project", "err", err)
		}
		summary, err := opts.Store.IndexSummary()
		if err != nil {
			slog.Error("webui index summary", "err", err)
			http.Error(w, "scans error", http.StatusInternalServerError)
			return
		}
		resp := ScansResponse{
			IndexSummary:   summary,
			IndexScans:     indexScans,
			DedupRuns:      dedupRuns,
			DedupTotals:    totals,
			DedupByProject: byProject,
			DedupEnabled:   opts.DedupEnabled,
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}
}

func enrichFlatSearchResults(ctx context.Context, opts Opts, results []storage.ArtifactRecord) {
	for i := range results {
		storage.EnrichArtifactRecord(&results[i], opts.ScanPaths)
		storage.EnrichArtifactComment(&results[i])
		sources, count, err := opts.Store.ListSourcesForBuildIDUI(ctx, results[i].BuildID, opts.ScanPaths, 20)
		if err != nil {
			continue
		}
		results[i].Sources = sources
		results[i].SourcesCount = count
	}
}

// RescanResponse — ответ на запуск scan из UI.
type RescanResponse struct {
	Status string `json:"status"`
}

func rescanHandler(opts Opts) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if !opts.ScanEnabled {
			http.Error(w, "scan disabled", http.StatusConflict)
			return
		}
		if opts.ScanTrigger == nil {
			http.Error(w, "scan trigger unavailable", http.StatusServiceUnavailable)
			return
		}
		opts.ScanTrigger.Trigger()
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(RescanResponse{Status: "accepted"})
	}
}
