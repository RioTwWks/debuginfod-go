package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/your-username/debuginfod-go/internal/cache"
	"github.com/your-username/debuginfod-go/internal/config"
	"github.com/your-username/debuginfod-go/internal/federation"
	"github.com/your-username/debuginfod-go/internal/indexer"
	"github.com/your-username/debuginfod-go/internal/logging"
	"github.com/your-username/debuginfod-go/internal/metrics"
	"github.com/your-username/debuginfod-go/internal/storage"
	"github.com/your-username/debuginfod-go/internal/webapi"
)

func main() {
	cfg := config.Load()
	logging.Setup(cfg.LogLevel)

	if err := os.MkdirAll(cfg.CacheDir, 0o755); err != nil {
		slog.Error("create cache dir", "err", err)
		os.Exit(1)
	}

	store, err := storage.Open(cfg.DBPath, cfg.DatabaseURL)
	if err != nil {
		slog.Error("open database", "err", err)
		os.Exit(1)
	}
	defer store.Close()

	collector := metrics.New()
	fed := federation.New(cfg.UpstreamURLs, 30*time.Second)

	idx := indexer.NewIndexer(indexer.Options{
		Storage:       store,
		Paths:         cfg.ScanPaths,
		CacheDir:      cfg.CacheDir,
		CacheMaxBytes: cfg.CacheMaxBytes,
		Workers:       cfg.ScanWorkers,
		Metrics:       collector,
	})
	go runIndexer(idx, cfg.RescanInterval)

	cacheBytes := func() int64 {
		n, _ := cache.DirSize(cfg.CacheDir)
		return n
	}

	opts := webapi.ServerOpts{
		Store:           store,
		MetadataMaxTime: cfg.MetadataMaxTime,
		Federation:      fed,
		Metrics:         collector,
		ZabbixKey:       cfg.ZabbixKey,
		CacheBytes:      cacheBytes,
	}

	server := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           webapi.NewMux(opts),
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		slog.Info("debuginfod started",
			"port", cfg.Port,
			"scan_paths", cfg.ScanPaths,
			"workers", cfg.ScanWorkers,
			"metadata_maxtime", cfg.MetadataMaxTime,
			"federation", len(cfg.UpstreamURLs) > 0,
		)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("server failed", "err", err)
			os.Exit(1)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		slog.Error("shutdown", "err", err)
	}
	slog.Info("server stopped")
}

func runIndexer(idx *indexer.Indexer, interval time.Duration) {
	if err := idx.Scan(); err != nil {
		slog.Error("index scan", "err", err)
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for range ticker.C {
		if err := idx.Scan(); err != nil {
			slog.Error("index scan", "err", err)
		}
	}
}
