package metrics

import (
	"sync"
	"sync/atomic"
	"time"
)

// ScanPhase — этап фонового scan.
type ScanPhase string

const (
	ScanPhaseIdle     ScanPhase = "idle"
	ScanPhaseIndexing ScanPhase = "indexing"
	ScanPhaseDedup    ScanPhase = "dedup"
)

// ScanStats — результат последнего прохода индексатора.
type ScanStats struct {
	Duration time.Duration
	Indexed  int
	Skipped  int
	Errors   int
	Finished time.Time
}

// ScanProgress — снимок хода выполнения scan/dedup для Web UI.
type ScanProgress struct {
	Running     bool      `json:"running"`
	Phase       ScanPhase `json:"phase"`
	StartedAt   time.Time `json:"started_at,omitempty"`
	Indexed     int64     `json:"indexed"`
	Skipped     int64     `json:"skipped"`
	Errors      int64     `json:"errors"`
	CurrentPath string    `json:"current_path,omitempty"`

	DedupGroupsTotal     int   `json:"dedup_groups_total"`
	DedupGroupsProcessed int   `json:"dedup_groups_processed"`
	DedupFilesCompressed int   `json:"dedup_files_compressed"`
	DedupFilesSkipped    int   `json:"dedup_files_skipped"`
	DedupErrors          int   `json:"dedup_errors"`
	DedupBytesBefore     int64 `json:"dedup_bytes_before"`
	DedupBytesAfter      int64 `json:"dedup_bytes_after"`
}

// Collector собирает runtime-метрики для Zabbix.
type Collector struct {
	startedAt time.Time

	httpRequests atomic.Uint64
	http2xx      atomic.Uint64
	http4xx      atomic.Uint64
	http5xx      atomic.Uint64
	bytesSent    atomic.Uint64

	federationHits   atomic.Uint64
	federationMisses atomic.Uint64

	lastScan atomic.Value // ScanStats
	ready    atomic.Bool

	scanRunning  atomic.Bool
	scanPhase    atomic.Value // ScanPhase
	scanStarted  atomic.Int64 // unix nano
	scanIndexed  atomic.Int64
	scanSkipped  atomic.Int64
	scanErrors   atomic.Int64
	scanPath     atomic.Value // string

	dedupGroupsTotal     atomic.Int64
	dedupGroupsProcessed atomic.Int64
	dedupFilesCompressed atomic.Int64
	dedupFilesSkipped    atomic.Int64
	dedupErrors          atomic.Int64
	dedupBytesBefore     atomic.Int64
	dedupBytesAfter      atomic.Int64

	pathMu         sync.Mutex
	lastPathUpdate time.Time
}

// New создаёт коллектор метрик.
func New() *Collector {
	c := &Collector{startedAt: time.Now()}
	c.lastScan.Store(ScanStats{})
	c.scanPhase.Store(ScanPhaseIdle)
	c.scanPath.Store("")
	return c
}

// RecordHTTP учитывает HTTP-ответ.
func (c *Collector) RecordHTTP(status int, bytes int64) {
	c.httpRequests.Add(1)
	c.bytesSent.Add(uint64(max64(bytes, 0)))
	switch {
	case status >= 500:
		c.http5xx.Add(1)
	case status >= 400:
		c.http4xx.Add(1)
	case status >= 200 && status < 300:
		c.http2xx.Add(1)
	}
}

// RecordScan сохраняет статистику индексации.
func (c *Collector) RecordScan(stats ScanStats) {
	c.lastScan.Store(stats)
	c.ready.Store(true)
}

// MarkReady помечает сервис готовым (например, при отключённом scan).
func (c *Collector) MarkReady() {
	c.ready.Store(true)
}

// Ready возвращает true после первого завершённого scan или MarkReady.
func (c *Collector) Ready() bool {
	return c.ready.Load()
}

// RecordFederationHit — артефакт найден на upstream-сервере.
func (c *Collector) RecordFederationHit() {
	c.federationHits.Add(1)
}

// RecordFederationMiss — upstream не вернул артефакт.
func (c *Collector) RecordFederationMiss() {
	c.federationMisses.Add(1)
}

// Uptime возвращает время работы процесса.
func (c *Collector) Uptime() time.Duration {
	return time.Since(c.startedAt)
}

// LastScan возвращает статистику последнего scan.
func (c *Collector) LastScan() ScanStats {
	if v := c.lastScan.Load(); v != nil {
		return v.(ScanStats)
	}
	return ScanStats{}
}

// BeginScan помечает начало фонового scan.
func (c *Collector) BeginScan(phase ScanPhase) {
	c.scanRunning.Store(true)
	c.scanPhase.Store(phase)
	c.scanStarted.Store(time.Now().UnixNano())
	c.scanIndexed.Store(0)
	c.scanSkipped.Store(0)
	c.scanErrors.Store(0)
	c.scanPath.Store("")
	c.dedupGroupsTotal.Store(0)
	c.dedupGroupsProcessed.Store(0)
	c.dedupFilesCompressed.Store(0)
	c.dedupFilesSkipped.Store(0)
	c.dedupErrors.Store(0)
	c.dedupBytesBefore.Store(0)
	c.dedupBytesAfter.Store(0)
}

// SetScanPhase переключает этап scan (indexing → dedup).
func (c *Collector) SetScanPhase(phase ScanPhase) {
	c.scanPhase.Store(phase)
}

// EndScan сбрасывает индикатор выполнения scan.
func (c *Collector) EndScan() {
	c.scanRunning.Store(false)
	c.scanPhase.Store(ScanPhaseIdle)
	c.scanPath.Store("")
}

// UpdateIndexingProgress публикует счётчики индексатора.
func (c *Collector) UpdateIndexingProgress(indexed, skipped, errors int64) {
	c.scanIndexed.Store(indexed)
	c.scanSkipped.Store(skipped)
	c.scanErrors.Store(errors)
}

// SetScanCurrentPath обновляет текущий файл (не чаще раза в 500 ms).
func (c *Collector) SetScanCurrentPath(path string) {
	c.pathMu.Lock()
	defer c.pathMu.Unlock()
	now := time.Now()
	if !c.lastPathUpdate.IsZero() && now.Sub(c.lastPathUpdate) < 500*time.Millisecond {
		return
	}
	c.lastPathUpdate = now
	c.scanPath.Store(path)
}

// SetDedupGroupsTotal задаёт число групп dedup для прогресса.
func (c *Collector) SetDedupGroupsTotal(total int) {
	c.dedupGroupsTotal.Store(int64(total))
}

// UpdateDedupProgress публикует счётчики dedup ingest.
func (c *Collector) UpdateDedupProgress(groupsDone, compressed, skipped, errors int, bytesBefore, bytesAfter int64) {
	c.dedupGroupsProcessed.Store(int64(groupsDone))
	c.dedupFilesCompressed.Store(int64(compressed))
	c.dedupFilesSkipped.Store(int64(skipped))
	c.dedupErrors.Store(int64(errors))
	c.dedupBytesBefore.Store(bytesBefore)
	c.dedupBytesAfter.Store(bytesAfter)
}

// ScanProgress возвращает снимок хода выполнения scan.
func (c *Collector) ScanProgress() ScanProgress {
	phase := ScanPhaseIdle
	if v := c.scanPhase.Load(); v != nil {
		phase = v.(ScanPhase)
	}
	path := ""
	if v := c.scanPath.Load(); v != nil {
		path = v.(string)
	}
	var startedAt time.Time
	if ns := c.scanStarted.Load(); ns > 0 {
		startedAt = time.Unix(0, ns)
	}
	return ScanProgress{
		Running:              c.scanRunning.Load(),
		Phase:                phase,
		StartedAt:            startedAt,
		Indexed:              c.scanIndexed.Load(),
		Skipped:              c.scanSkipped.Load(),
		Errors:               c.scanErrors.Load(),
		CurrentPath:          path,
		DedupGroupsTotal:     int(c.dedupGroupsTotal.Load()),
		DedupGroupsProcessed: int(c.dedupGroupsProcessed.Load()),
		DedupFilesCompressed: int(c.dedupFilesCompressed.Load()),
		DedupFilesSkipped:    int(c.dedupFilesSkipped.Load()),
		DedupErrors:          int(c.dedupErrors.Load()),
		DedupBytesBefore:     c.dedupBytesBefore.Load(),
		DedupBytesAfter:      c.dedupBytesAfter.Load(),
	}
}

func (c *Collector) HTTPRequests() uint64      { return c.httpRequests.Load() }
func (c *Collector) HTTP2xx() uint64           { return c.http2xx.Load() }
func (c *Collector) HTTP4xx() uint64           { return c.http4xx.Load() }
func (c *Collector) HTTP5xx() uint64           { return c.http5xx.Load() }
func (c *Collector) BytesSent() uint64         { return c.bytesSent.Load() }
func (c *Collector) FederationHits() uint64    { return c.federationHits.Load() }
func (c *Collector) FederationMisses() uint64  { return c.federationMisses.Load() }

func max64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}
