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
		LazyExtract:   cfg.LazyExtract,
	})
	go runIndexer(idx, cfg.RescanInterval)

	cacheBytes := func() int64 {
		n, _ := cache.DirSize(cfg.CacheDir)
		return n
	}

	security := webapi.SecurityOpts{
		CORSOrigins:   cfg.CORSOrigins,
		RateLimitRPS:  cfg.RateLimitRPS,
		BasicAuthUser: cfg.BasicAuthUser,
		BasicAuthPass: cfg.BasicAuthPass,
		TLSCertFile:   cfg.TLSCertFile,
		TLSKeyFile:    cfg.TLSKeyFile,
		TLSClientCA:   cfg.TLSClientCA,
	}

	opts := webapi.ServerOpts{
		Store:            store,
		MetadataMaxTime:  cfg.MetadataMaxTime,
		MetadataPageSize: cfg.MetadataPageSize,
		Federation:       fed,
		Metrics:          collector,
		ZabbixKey:        cfg.ZabbixKey,
		CacheBytes:       cacheBytes,
		CacheDir:         cfg.CacheDir,
		ScanPaths:        cfg.ScanPaths,
		UIEnabled:        cfg.UIEnabled,
		Security:         security,
	}

	tlsConfig, err := security.BuildTLSConfig()
	if err != nil {
		slog.Error("tls config", "err", err)
		os.Exit(1)
	}

	server := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           webapi.NewMux(opts),
		ReadHeaderTimeout: 10 * time.Second,
		TLSConfig:         tlsConfig,
	}

	go func() {
		slog.Info("debuginfod started",
			"port", cfg.Port,
			"tls", security.TLSConfigured(),
			"mtls", security.TLSClientCA != "",
			"basic_auth", security.BasicAuthEnabled(),
			"cors", len(cfg.CORSOrigins) > 0,
			"rate_limit", cfg.RateLimitRPS,
			"scan_paths", cfg.ScanPaths,
			"workers", cfg.ScanWorkers,
			"federation", len(cfg.UpstreamURLs) > 0,
		)
		var serveErr error
		if security.TLSConfigured() {
			serveErr = server.ListenAndServeTLS(cfg.TLSCertFile, cfg.TLSKeyFile)
		} else {
			serveErr = server.ListenAndServe()
		}
		if serveErr != nil && serveErr != http.ErrServerClosed {
			slog.Error("server failed", "err", serveErr)
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
