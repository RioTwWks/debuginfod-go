package metrics

import (
	"sync/atomic"
	"time"
)

// ScanStats — результат последнего прохода индексатора.
type ScanStats struct {
	Duration  time.Duration
	Indexed   int
	Skipped   int
	Errors    int
	Finished  time.Time
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
}

// New создаёт коллектор метрик.
func New() *Collector {
	c := &Collector{startedAt: time.Now()}
	c.lastScan.Store(ScanStats{})
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

func (c *Collector) HTTPRequests() uint64  { return c.httpRequests.Load() }
func (c *Collector) HTTP2xx() uint64       { return c.http2xx.Load() }
func (c *Collector) HTTP4xx() uint64       { return c.http4xx.Load() }
func (c *Collector) HTTP5xx() uint64       { return c.http5xx.Load() }
func (c *Collector) BytesSent() uint64     { return c.bytesSent.Load() }
func (c *Collector) FederationHits() uint64   { return c.federationHits.Load() }
func (c *Collector) FederationMisses() uint64 { return c.federationMisses.Load() }

func max64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}
