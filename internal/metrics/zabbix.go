package metrics

import (
	"encoding/json"
	"net/http"

	"github.com/your-username/debuginfod-go/internal/storage"
)

// DBStatsProvider возвращает счётчики из БД.
type DBStatsProvider interface {
	Stats() (storage.Stats, error)
}

// ZabbixPayload — JSON для HTTP agent Zabbix (JSONPath).
type ZabbixPayload struct {
	UptimeSeconds       int64 `json:"uptime_seconds"`
	ArtifactsTotal      int64 `json:"artifacts_total"`
	ArtifactsExecutable int64 `json:"artifacts_executable"`
	ArtifactsDebuginfo  int64 `json:"artifacts_debuginfo"`
	SourcesTotal        int64 `json:"sources_total"`
	ScannedFilesTotal   int64 `json:"scanned_files_total"`
	LastScanDurationMs  int64 `json:"last_scan_duration_ms"`
	LastScanIndexed     int   `json:"last_scan_indexed"`
	LastScanSkipped     int   `json:"last_scan_skipped"`
	LastScanErrors      int   `json:"last_scan_errors"`
	HTTPRequestsTotal   uint64 `json:"http_requests_total"`
	HTTP2xxTotal        uint64 `json:"http_2xx_total"`
	HTTP4xxTotal        uint64 `json:"http_4xx_total"`
	HTTP5xxTotal        uint64 `json:"http_5xx_total"`
	HTTPBytesSent       uint64 `json:"http_bytes_sent"`
	FederationHits      uint64 `json:"federation_hits"`
	FederationMisses    uint64 `json:"federation_misses"`
	CacheBytes          int64  `json:"cache_bytes"`
}

// Handler отдаёт метрики для Zabbix HTTP agent.
func Handler(collector *Collector, db DBStatsProvider, cacheBytes func() int64, authKey string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if authKey != "" && r.Header.Get("X-Zabbix-Token") != authKey && r.URL.Query().Get("key") != authKey {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		dbStats, err := db.Stats()
		if err != nil {
			http.Error(w, "stats error", http.StatusInternalServerError)
			return
		}

		scan := collector.LastScan()
		payload := ZabbixPayload{
			UptimeSeconds:       int64(collector.Uptime().Seconds()),
			ArtifactsTotal:      dbStats.ArtifactsTotal,
			ArtifactsExecutable: dbStats.ArtifactsExecutable,
			ArtifactsDebuginfo:  dbStats.ArtifactsDebuginfo,
			SourcesTotal:        dbStats.SourcesTotal,
			ScannedFilesTotal:   dbStats.ScannedFilesTotal,
			LastScanDurationMs:  scan.Duration.Milliseconds(),
			LastScanIndexed:     scan.Indexed,
			LastScanSkipped:     scan.Skipped,
			LastScanErrors:      scan.Errors,
			HTTPRequestsTotal:   collector.HTTPRequests(),
			HTTP2xxTotal:        collector.HTTP2xx(),
			HTTP4xxTotal:        collector.HTTP4xx(),
			HTTP5xxTotal:        collector.HTTP5xx(),
			HTTPBytesSent:       collector.BytesSent(),
			FederationHits:      collector.FederationHits(),
			FederationMisses:      collector.FederationMisses(),
			CacheBytes:          cacheBytes(),
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(payload)
	}
}
