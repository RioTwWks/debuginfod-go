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
	CacheBytes          int64  `json:"cache_bytes"`
	ScanEnabled         bool   `json:"scan_enabled"`
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
		}
		if !scan.Finished.IsZero() {
			resp.LastScanFinishedAt = scan.Finished.UTC().Format(time.RFC3339)
		}
		if opts.CacheBytes != nil {
			resp.CacheBytes = opts.CacheBytes()
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
			grouped, err := opts.Store.SearchBuildIDGroupedForUI(ctx, query, limit)
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
