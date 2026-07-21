package webapi

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/your-username/debuginfod-go/internal/federation"
	"github.com/your-username/debuginfod-go/internal/metrics"
	"github.com/your-username/debuginfod-go/internal/pathsafe"
	"github.com/your-username/debuginfod-go/internal/storage"
	"github.com/your-username/debuginfod-go/internal/webui"
	"github.com/your-username/debuginfod-go/pkg/buildid"
	"github.com/your-username/debuginfod-go/pkg/elfsection"
)

// ServerOpts — зависимости HTTP-слоя.
type ServerOpts struct {
	Store            *storage.Storage
	MetadataMaxTime  time.Duration
	MetadataPageSize int
	Federation       *federation.Client
	Metrics          *metrics.Collector
	ZabbixKey        string
	AdminKey         string
	ScanTrigger      ScanTrigger
	DedupRunner      DedupRunner
	DedupRestorer    DedupRestorer
	CacheBytes       func() int64
	CacheDir         string
	ScanPaths        []string
	UIEnabled        bool
	DedupEnabled     bool
	Security         SecurityOpts
}

// Handler обслуживает HTTP-запросы протокола debuginfod.
type Handler struct {
	store         *storage.Storage
	federation    *federation.Client
	metrics       *metrics.Collector
	cacheDir      string
	allowedRoots  []string
	dedupRestorer DedupRestorer
}

// NewHandler создаёт HTTP-обработчик.
func NewHandler(opts ServerOpts) *Handler {
	return &Handler{
		store:         opts.Store,
		federation:    opts.Federation,
		metrics:       opts.Metrics,
		cacheDir:      opts.CacheDir,
		allowedRoots:  pathsafe.AllowedRoots(opts.ScanPaths, opts.CacheDir),
		dedupRestorer: opts.DedupRestorer,
	}
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
		if err := pathsafe.ValidateHTTPSourcePath(sourcePath); err != nil {
			http.Error(w, "invalid source path", http.StatusBadRequest)
			return
		}
		h.serveSource(w, r, buildID, sourcePath)
	case "section":
		if len(parts) < 4 {
			http.Error(w, "section name required", http.StatusBadRequest)
			return
		}
		sectionName := parts[3]
		if err := pathsafe.ValidateSectionName(sectionName); err != nil {
			http.Error(w, "invalid section name", http.StatusBadRequest)
			return
		}
		h.serveSection(w, r, buildID, sectionName)
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

		offset, limit := parseMetadataPagination(r, opts.MetadataPageSize)

		ctx := r.Context()
		if opts.MetadataMaxTime > 0 {
			var cancel context.CancelFunc
			ctx, cancel = context.WithTimeout(ctx, opts.MetadataMaxTime)
			defer cancel()
		}

		resp, err := opts.Store.SearchMetadataQuery(ctx, storage.MetadataQuery{
			Key: key, Value: value, Offset: offset, Limit: limit,
		})
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

func parseMetadataPagination(r *http.Request, defaultLimit int) (offset, limit int) {
	if v := r.URL.Query().Get("offset"); v != "" {
		offset, _ = strconv.Atoi(v)
		if offset < 0 {
			offset = 0
		}
	}
	if v := r.URL.Query().Get("limit"); v != "" {
		limit, _ = strconv.Atoi(v)
	} else if r.URL.Query().Has("offset") {
		limit = defaultLimit
	}
	if limit < 0 {
		limit = 0
	}
	if limit > 1000 {
		limit = 1000
	}
	if defaultLimit <= 0 {
		defaultLimit = 100
	}
	if r.URL.Query().Has("offset") && limit == 0 {
		limit = defaultLimit
	}
	return offset, limit
}

func (h *Handler) serveArtifact(w http.ResponseWriter, r *http.Request, buildID, artifactType string) {
	loc, err := h.store.GetArtifactLocation(buildID, artifactType)
	if errors.Is(err, storage.ErrNotFound) {
		h.tryFederation(w, r)
		return
	}
	if err != nil {
		logResolveError("GetArtifactLocation", err, "build_id", buildID, "type", artifactType)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	if loc.FilePath == "" && loc.ArchivePath != "" {
		if err := h.validateArchiveAccess(loc.ArchivePath, loc.MemberPath); err != nil {
			logResolveError("validateArchive", err, "build_id", buildID)
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		if err := streamMember(w, loc.ArchivePath, loc.MemberPath); err != nil {
			logResolveError("streamArtifact", err, "build_id", buildID, "type", artifactType)
			http.Error(w, "internal error", http.StatusInternalServerError)
		}
		return
	}

	path, err := resolveFilePathWithDedup(h.cacheDir, loc, h.dedupRestorer)
	if err != nil {
		logResolveError("resolveArtifact", err, "build_id", buildID)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if err := h.validateServingPath(path); err != nil {
		logResolveError("validateArtifactPath", err, "build_id", buildID, "path", path)
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	serveResolvedFile(w, r, path, artifactDownloadName(loc))
}

func artifactDownloadName(loc storage.ArtifactLocation) string {
	if loc.MemberPath != "" {
		return filepath.Base(loc.MemberPath)
	}
	if loc.FilePath != "" {
		return filepath.Base(loc.FilePath)
	}
	return ""
}

func (h *Handler) validateArchiveAccess(archivePath, memberPath string) error {
	if err := pathsafe.ValidateMemberPath(memberPath); err != nil {
		return err
	}
	return pathsafe.ValidateArchivePath(archivePath, h.allowedRoots)
}

func (h *Handler) validateArtifactLocation(loc storage.ArtifactLocation) error {
	if loc.FilePath != "" {
		return h.validateServingPath(loc.FilePath)
	}
	if loc.ArchivePath != "" {
		return h.validateArchiveAccess(loc.ArchivePath, loc.MemberPath)
	}
	return nil
}

func (h *Handler) validateServingPath(path string) error {
	return pathsafe.AssertUnderRoots(path, h.allowedRoots)
}

func (h *Handler) serveSource(w http.ResponseWriter, r *http.Request, buildID, sourcePath string) {
	loc, err := h.store.GetSource(buildID, sourcePath)
	if errors.Is(err, storage.ErrNotFound) {
		h.tryFederation(w, r)
		return
	}
	if err != nil {
		logResolveError("GetSource", err, "build_id", buildID, "path", sourcePath)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	path, err := resolveSourcePath(h.cacheDir, loc)
	if err != nil {
		logResolveError("resolveSource", err, "build_id", buildID, "path", sourcePath)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if err := h.validateServingPath(path); err != nil {
		logResolveError("validateSourcePath", err, "build_id", buildID, "path", path)
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	serveResolvedFile(w, r, path, filepath.Base(sourcePath))
}

func (h *Handler) serveSection(w http.ResponseWriter, r *http.Request, buildID, sectionName string) {
	debuginfo, executable, err := h.store.GetArtifactPaths(buildID)
	if err != nil {
		logResolveError("GetArtifactPaths", err, "build_id", buildID)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if debuginfo.FilePath == "" && debuginfo.ArchivePath == "" &&
		executable.FilePath == "" && executable.ArchivePath == "" {
		h.tryFederation(w, r)
		return
	}

	if err := h.validateArtifactLocation(debuginfo); err != nil {
		logResolveError("validateDebuginfo", err, "build_id", buildID)
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	if err := h.validateArtifactLocation(executable); err != nil {
		logResolveError("validateExecutable", err, "build_id", buildID)
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	data, err := extractSectionFromLocations(h.cacheDir, debuginfo, executable, sectionName, h.dedupRestorer)
	if errors.Is(err, elfsection.ErrNotFound) {
		h.tryFederation(w, r)
		return
	}
	if err != nil {
		logResolveError("ExtractSection", err, "build_id", buildID, "section", sectionName)
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
	mux.HandleFunc("/readyz", ReadyHandler(opts.Metrics))
	mux.HandleFunc("/admin/rescan", AdminRescanHandler(opts.ScanTrigger, opts.AdminKey))
	mux.HandleFunc("/admin/dedup-backfill", AdminDedupBackfillHandler(opts.DedupRunner, opts.AdminKey))
	mux.HandleFunc("/metadata", MetadataHandler(opts))
	mux.HandleFunc("/openapi.yaml", OpenAPIHandler)
	mux.HandleFunc("/zabbix", metrics.Handler(opts.Metrics, opts.Store, opts.CacheBytes, opts.ZabbixKey))
	mux.Handle("/buildid/", NewHandler(opts))

	if opts.UIEnabled {
		webui.Register(mux, webui.Opts{
			Store:        opts.Store,
			Metrics:      opts.Metrics,
			CacheBytes:   opts.CacheBytes,
			ScanPaths:    opts.ScanPaths,
			DedupEnabled: opts.DedupEnabled,
			ScanEnabled:  opts.ScanTrigger != nil,
			ScanTrigger:  opts.ScanTrigger,
		})
	}

	var handler http.Handler = mux
	if opts.Metrics != nil {
		handler = MetricsMiddleware(opts.Metrics, handler)
	}
	handler = GzipMiddleware(handler)
	handler = BasicAuthMiddleware(opts.Security.BasicAuthUser, opts.Security.BasicAuthPass, handler)
	handler = RateLimitMiddleware(opts.Security.RateLimitRPS, handler)
	handler = CORSMiddleware(opts.Security.CORSOrigins, handler)
	return handler
}
